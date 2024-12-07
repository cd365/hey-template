package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"root/utils"
	"root/values"
	"strings"
	"text/template"
	"time"
	"unsafe"

	"github.com/cd365/hey/v2"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

const (
	byte34              = '"'
	byte96              = '`'
	templateLeft        = "{{{"
	templateRight       = "}}}"
	tableFilenamePrefix = "zzz_"
	tableFilenameSuffix = "_aaa"
	tableFilenameGo     = ".go"
)

type Helper interface {
	QueryAllTable() error
	GetAllTable() []*SchemaTable
	QueryTableDefineSql(table *SchemaTable) error
}

type App struct {
	Version string

	cfg *Config

	way *hey.Way // 数据库连接对象

	helper Helper // 数据接口
}

func NewApp(
	ctx context.Context,
	cfg *Config,
) *App {
	_ = ctx
	return &App{
		Version: values.Version,
		cfg:     cfg,
	}
}

func (s *App) initial() error {
	cfg := s.cfg
	cfg.Driver = strings.TrimSpace(cfg.Driver)
	way, err := hey.NewWay(cfg.Driver, cfg.DataSourceName)
	if err != nil {
		return err
	}
	s.way = way
	db := way.DB()
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(2)
	db.SetConnMaxIdleTime(time.Minute * 3)
	db.SetConnMaxLifetime(time.Minute * 3)
	switch cfg.Driver {
	case hey.DriverNameMysql:
		cfg.DatabaseIdentify = "`"
		s.helper = NewMysql(s)
		if cfg.TableSchemaName == "" {
			start := strings.Index(cfg.DataSourceName, "/")
			if start > -1 {
				end := strings.Index(cfg.DataSourceName, "?")
				if end > -1 {
					cfg.TableSchemaName = cfg.DataSourceName[start+1 : end]
				} else {
					cfg.TableSchemaName = cfg.DataSourceName[start+1:]
				}
			}
			cfg.TableSchemaName = strings.TrimSpace(cfg.TableSchemaName)
		}
	case hey.DriverNamePostgres:
		cfg.DatabaseIdentify = `"`
		s.helper = NewPgsql(s)
		if cfg.TableSchemaName == "" {
			cfg.TableSchemaName = "public"
		}
	default:
		return fmt.Errorf("unsupported driver name: %s", cfg.Driver)
	}
	return nil
}

func (s *App) writeFile(reader io.Reader, filename string) error {
	fil, err := utils.RemoveCreateFile(filename)
	if err != nil {
		return err
	}
	defer func() { _ = fil.Close() }()
	_, err = io.Copy(fil, reader)
	return err
}

func (s *App) getAllTable(all bool) []*SchemaTable {
	if all {
		return s.helper.GetAllTable()
	}
	allTable := s.helper.GetAllTable()
	length := len(allTable)
	result := make([]*SchemaTable, 0, length)
	for i := 0; i < length; i++ {
		if s.cfg.Disable(*allTable[i].TableName) {
			continue
		}
		result = append(result, allTable[i])
	}
	return result
}

func (s *App) BuildAll() error {
	if err := s.initial(); err != nil {
		return err
	}
	if s.cfg.Driver == hey.DriverNamePostgres {
		if _, err := s.way.DB().Exec(pgsqlFuncCreate); err != nil {
			return err
		}
		defer func() { _, _ = s.way.DB().Exec(pgsqlFuncDrop) }()
	}
	if err := s.helper.QueryAllTable(); err != nil {
		return err
	}
	for _, table := range s.getAllTable(true) {
		if err := s.helper.QueryTableDefineSql(table); err != nil {
			return err
		}
	}
	writer := make([]func() error, 0, 8)
	writer = append(writer, s.Model)
	for _, w := range writer {
		if err := w(); err != nil {
			return err
		}
	}
	return nil
}

type TableColumnPrimaryKey struct {
	OriginNamePascal      string // 表名(帕斯卡命名)
	PrimaryKeyPascal      string // 主键名(帕斯卡命名)
	PrimaryKeySmallPascal string // 主键名(驼峰命名)
	PrimaryKeyUpper       string // 主键名(全大写) 如: ACCOUNT_USERNAME
	PrimaryKeyType        string // 主键在go语言里面的类型(int | int64 | string), 其它类型无效
}

type TmplTableModel struct {
	table *SchemaTable

	Version string // 模板版本

	OriginName           string // 原始表名称
	OriginNamePascal     string // 原始表名称(帕斯卡命名)
	OriginNameWithPrefix string // 原始表名称
	OriginNameCamel      string // 表名(帕斯卡命名)首字母小写表名
	Comment              string // 表注释(如果表没有注释使用原始表名作为默认值)

	// model
	StructColumn                      []string // 表结构体字段定义 ==> Name string `json:"name" db:"name"` // 名称
	StructColumnSchema                []string // 表结构体字段关系定义 ==> Name string // name 名称
	StructColumnSchemaFieldSlice      string   // NewHey.Field ==> // []string{"id", "name"}
	StructColumnSchemaFieldSliceValue string   // NewHey.FieldStr ==> // `"id", "name"` || "`id`, `name`"
	StructColumnPrimaryKey            string   // 表结构体字段定义 ==> 主键字段结构体定义
	StructColumnMod                   []string // 表结构体字段定义 ==> Name *string `json:"name" db:"name"` // 名称
	StructColumnAdd                   []string // 表结构体字段定义 ==> Name *string `json:"name" db:"name"` // 名称
	StructColumnAddPrimaryKey         string   // 表结构体字段定义 ==> 添加结构体设置 PrimaryKey 方法

	StructColumnSchemaValues          []string // NewHey.Attribute ==> Name:"name", // 名称
	StructColumnSchemaValuesAccess    string   // NewHey.Access ==> Access:[]string{}, // 访问字段列表
	StructColumnSchemaValuesAccessMap string   // NewHey.AccessMap ==> Access:map[string]struct{}, // 访问字段列表

	ColumnAutoIncr  string // 结构体字段方法 ColumnAutoIncr
	ColumnCreatedAt string // 结构体字段方法 ColumnCreatedAt
	ColumnUpdatedAt string // 结构体字段方法 ColumnUpdatedAt
	ColumnDeletedAt string // 结构体字段方法 ColumnDeletedAt

	PrimaryKey string // 主键自定义方法
}

func (s *TmplTableModel) prepare() error {

	// struct define
	for i, c := range s.table.Column {
		tmp := fmt.Sprintf("\t%s %s `json:\"%s\" db:\"%s\"`",
			c.pascal(),
			c.databaseTypeToGoType(),
			*c.ColumnName,
			*c.ColumnName,
		)
		comment := c.comment()
		if comment != "" {
			tmp = fmt.Sprintf("%s // %s", tmp, comment)
		}
		if i != 0 {
			tmp = fmt.Sprintf("\n%s", tmp)
		}
		s.StructColumn = append(s.StructColumn, tmp)
	}

	// schema
	for i, c := range s.table.Column {
		tmp := fmt.Sprintf("\t%s string", strings.ToUpper(*c.ColumnName))
		comment := c.comment()
		if comment != "" {
			tmp = fmt.Sprintf("%s // %s", tmp, comment)
		}
		if i != 0 {
			tmp = fmt.Sprintf("\n%s", tmp)
		}
		s.StructColumnSchema = append(s.StructColumnSchema, tmp)
	}

	// column list
	{
		lengthColumn := len(s.table.Column)
		field := make([]string, 0, lengthColumn)
		fieldAccess := make([]string, 0, lengthColumn)
		for i, c := range s.table.Column {
			field = append(field, fmt.Sprintf("\"%s\"", *c.ColumnName))
			upperName := c.upper()
			fieldAccessTmp := fmt.Sprintf("s.%s,", upperName)
			if c.ColumnComment != nil {
				fieldAccessTmp = fmt.Sprintf("%s // %s", fieldAccessTmp, *c.ColumnComment)
			}
			fieldAccess = append(fieldAccess, fieldAccessTmp)
			tmp := fmt.Sprintf("\t\t%s:\"%s\",", upperName, *c.ColumnName)
			comment := c.comment()
			if comment != "" {
				tmp = fmt.Sprintf("%s // %s", tmp, comment)
			}
			if i != 0 {
				tmp = fmt.Sprintf("\n%s", tmp)
			}
			s.StructColumnSchemaValues = append(s.StructColumnSchemaValues, tmp)
		}

		{
			s96 := string(byte96) // `
			s34 := `"`            // "
			s.StructColumnSchemaFieldSlice = strings.Join(field, ", ")
			switch s.table.app.cfg.Driver {
			case hey.DriverNameMysql:
				s.StructColumnSchemaFieldSlice = strings.ReplaceAll(s.StructColumnSchemaFieldSlice, s34, s96)
			}
			if strings.Contains(s.StructColumnSchemaFieldSlice, s96) {
				s.StructColumnSchemaFieldSliceValue = hey.ConcatString(s34, s.StructColumnSchemaFieldSlice, s34)
			} else {
				s.StructColumnSchemaFieldSliceValue = hey.ConcatString(s96, s.StructColumnSchemaFieldSlice, s96)
			}
		}

		s.StructColumnSchemaValuesAccess = fmt.Sprintf("[]string{\n\t\t%s\n\t}", strings.Join(fieldAccess, "\n\t\t"))

		fieldAccessMap := fieldAccess[:]
		for k, v := range fieldAccessMap {
			fieldAccessMap[k] = strings.Replace(v, ",", ":{},", 1)
		}
		s.StructColumnSchemaValuesAccessMap = fmt.Sprintf("map[string]*struct{}{\n\t\t%s\n\t}", strings.Join(fieldAccessMap, "\n\t\t"))
	}

	// ignore columns, for insert and update
	var ignore []string

	// cannotBeUpdatedFieldsMap Field map that does not need to be updated
	cannotBeUpdatedFieldsMap := make(map[string]*struct{})

	// table special fields
	{
		// auto increment field or timestamp field
		cm := make(map[string]*SchemaColumn)
		for _, v := range s.table.Column {
			if v.ColumnName == nil || *v.ColumnName == "" {
				continue
			}
			// make sure the type is integer
			if !strings.Contains(v.databaseTypeToGoType(), "int") {
				continue
			}
			cm[*v.ColumnName] = v
		}
		fc := func(cols ...string) []string {
			tmp := make([]string, 0)
			for k, v := range cols {
				cols[k] = strings.TrimSpace(v)
				if _, ok := cm[v]; ok {
					tmp = append(tmp, v)
				}
			}
			return tmp
		}
		autoIncrement := fc(s.table.app.cfg.ColumnSerial) // auto increment column
		if s.table.TableFieldSerial != "" && s.table.app.cfg.ColumnSerial != s.table.TableFieldSerial {
			autoIncrement = append(autoIncrement, s.table.TableFieldSerial)
		}
		created := fc(strings.Split(s.table.app.cfg.ColumnCreatedAt, ",")...) // created_at columns
		updated := fc(strings.Split(s.table.app.cfg.ColumnUpdatedAt, ",")...) // updated_at columns
		deleted := fc(strings.Split(s.table.app.cfg.ColumnDeletedAt, ",")...) // deleted_at columns

		ignore = append(ignore, autoIncrement[:]...)
		ignore = append(ignore, created[:]...)

		for _, field := range ignore {
			cannotBeUpdatedFieldsMap[field] = &struct{}{}
		}

		ignore = append(ignore, updated[:]...)
		ignore = append(ignore, deleted[:]...)

		if len(autoIncrement) > 0 && autoIncrement[0] != "" {
			s.ColumnAutoIncr = fmt.Sprintf("[]string{ s.%s }", utils.Upper(autoIncrement[0]))
		} else {
			s.ColumnAutoIncr = "nil"
		}
		cs := func(cols ...string) string {
			length := len(cols)
			if length == 0 {
				return "nil"
			}
			for i := 0; i < length; i++ {
				cols[i] = fmt.Sprintf("s.%s", utils.Upper(cols[i]))
			}
			return fmt.Sprintf("[]string{ %s }", strings.Join(cols, ", "))
		}
		s.ColumnCreatedAt = cs(created...)
		s.ColumnUpdatedAt = cs(updated...)
		s.ColumnDeletedAt = cs(deleted...)
	}

	ignoreMap := make(map[string]struct{})
	for _, v := range ignore {
		ignoreMap[v] = struct{}{}
	}

	// add
	write := false
	for _, c := range s.table.Column {
		if s.table.TableFieldSerial == *c.ColumnName {
			s.StructColumnAddPrimaryKey = fmt.Sprintf("func (s INSERT%s) PrimaryKey() interface{} {\n\treturn nil\n}", s.table.pascal())
			continue
		}
		if _, ok := ignoreMap[*c.ColumnName]; ok {
			continue // ignore columns like id, created_at, updated_at, deleted_at
		}
		opts := ""
		if c.CharacterMaximumLength != nil && *c.CharacterOctetLength > 0 {
			opts = fmt.Sprintf(",min=0,max=%d", *c.CharacterMaximumLength)
		}
		tmp := fmt.Sprintf("\t%s %s `json:\"%s\" db:\"%s\" validate:\"omitempty%s\"`",
			c.pascal(),
			c.databaseTypeToGoType(),
			*c.ColumnName,
			*c.ColumnName,
			opts,
		)

		comment := c.comment()
		if comment != "" {
			tmp = fmt.Sprintf("%s // %s", tmp, comment)
		}
		if write {
			tmp = fmt.Sprintf("\n%s", tmp)
		} else {
			write = true
		}
		s.StructColumnAdd = append(s.StructColumnAdd, tmp)
	}

	// mod
	write = false
	delete(ignoreMap, s.table.TableFieldSerial)
	for _, c := range s.table.Column {
		if _, ok := ignoreMap[*c.ColumnName]; ok {
			continue // ignore columns like created_at, updated_at, deleted_at
		}
		opts := ""
		if c.CharacterMaximumLength != nil && *c.CharacterOctetLength > 0 {
			opts = fmt.Sprintf(",min=0,max=%d", *c.CharacterMaximumLength)
		}
		tmp := fmt.Sprintf("\t%s *%s `json:\"%s\" db:\"%s\" validate:\"omitempty%s\"`",
			c.pascal(),
			c.databaseTypeToGoType(),
			*c.ColumnName,
			*c.ColumnName,
			opts,
		)
		if *c.ColumnName == s.table.TableFieldSerial {
			// TableSerial
			// TableSerial.SERIAL
			comment := c.comment()
			if comment != "" {
				comment = fmt.Sprintf(" // %s", comment)
			}
			tablePascal := s.table.pascal()
			columnPascal := c.pascal()
			s.StructColumnPrimaryKey = fmt.Sprintf("type PRIMARYKEY%s struct {\n\t%s *%s `json:\"%s\" db:\"-\" validate:\"omitempty,min=1\"`%s\n}\n\nfunc (s PRIMARYKEY%s) PrimaryKey() interface{} {\n\t if s.%s != nil {\n\treturn *s.%s\n\t}\n\treturn nil\n}",
				tablePascal,
				columnPascal,
				c.databaseTypeToGoType(),
				utils.Underline(*c.ColumnName),
				comment,
				tablePascal,
				columnPascal,
				columnPascal,
			)
			// append Primary-Key define
			s.StructColumnMod = append(s.StructColumnMod, fmt.Sprintf("\tPRIMARYKEY%s\n", tablePascal))
			continue
		}

		comment := c.comment()
		if comment != "" {
			tmp = fmt.Sprintf("%s // %s", tmp, comment)
		}
		if write {
			tmp = fmt.Sprintf("\n%s", tmp)
		} else {
			write = true
		}
		s.StructColumnMod = append(s.StructColumnMod, tmp)
	}

	// primary key
	if s.table.TableFieldSerial != "" {
		tmpl := NewTemplate(
			fmt.Sprintf("tmpl_model_schema_content_primary_key_%s_%s", *s.table.TableName, s.table.TableFieldSerial),
			tmplModelSchemaContentPrimaryKey,
		)
		buffer := bytes.NewBuffer(nil)
		data := &TableColumnPrimaryKey{
			OriginNamePascal:      s.table.pascal(),
			PrimaryKeyPascal:      utils.Pascal(s.table.TableFieldSerial),
			PrimaryKeySmallPascal: utils.PascalFirstLower(s.table.TableFieldSerial),
			PrimaryKeyUpper:       strings.ToUpper(s.table.TableFieldSerial),
		}
		for _, c := range s.table.Column {
			if s.table.TableFieldSerial == *c.ColumnName {
				data.PrimaryKeyType = strings.ToLower(c.databaseTypeToGoType())
				break
			}
		}
		if err := tmpl.Execute(buffer, data); err != nil {
			return err
		} else {
			s.PrimaryKey = buffer.String()
		}
	}

	return nil
}

type TmplTableModelSchema struct {
	Version string // 模板版本

	PrefixPackage string // 包导入前缀

	// data
	MapListDefine  string // data_schema.go tables define
	MapListAssign  string // data_schema.go tables assign
	MapListStorage string // data_schema.go tables storage
	MapListSlice   string // data_schema.go tables slice
}

func (s *App) Model() error {
	tables := s.getAllTable(false)

	// model_schema.go
	tmpModelSchema := NewTemplate("tmpl_model_schema", tmplModelSchema)
	tmpModelSchemaCustom := NewTemplate("tmpl_model_schema_custom", tmplModelSchemaCustom)
	tmpModelSchemaContent := NewTemplate("tmpl_model_schema_content", tmplModelSchemaContent)
	modelSchemaFilename := pathJoin(s.cfg.TemplateOutputDirectory, "model", "model_schema.go")
	modelSchemaCustomFilename := pathJoin(s.cfg.TemplateOutputDirectory, "model", "model_schema_custom.go")
	modelSchemaBuffer := bytes.NewBuffer(nil)
	modelSchemaCustomBuffer := bytes.NewBuffer(nil)
	modelTableCreateFilename := pathJoin(s.cfg.TemplateOutputDirectory, "model", "aaa_table_create.sql")
	modelTableCreateBuffer := bytes.NewBuffer(nil)

	for index, table := range tables {
		// for table ddl
		{
			ddl := table.DDL
			for strings.HasSuffix(ddl, "\n") {
				ddl = strings.TrimSuffix(ddl, "\n")
			}
			if index > 0 {
				if _, err := modelTableCreateBuffer.WriteString("\n\n\n\n"); err != nil {
					return err
				}
			}
			// comment
			if _, err := modelTableCreateBuffer.WriteString(fmt.Sprintf("/* %s (%s) */\n", *table.TableName, *table.TableComment)); err != nil {
				return err
			}
			// add drop table sql
			dropTableName := fmt.Sprintf("%s%s%s", s.cfg.DatabaseIdentify, *table.TableName, s.cfg.DatabaseIdentify)
			if _, err := modelTableCreateBuffer.WriteString(fmt.Sprintf("DROP TABLE IF EXISTS %s;\n", dropTableName)); err != nil {
				return err
			}
			if _, err := modelTableCreateBuffer.WriteString(ddl); err != nil {
				return err
			}
			if !strings.HasSuffix(ddl, ";") {
				if _, err := modelTableCreateBuffer.WriteString(";"); err != nil {
					return err
				}
			}
		}
		// table
		modelSchemaContentBuffer := bytes.NewBuffer(nil)
		tmp, err := table.newTmplTableModel()
		if err != nil {
			return err
		}
		if err = tmpModelSchemaContent.Execute(modelSchemaContentBuffer, tmp); err != nil {
			return err
		}
		modelSchemaContentFilename := pathJoin(s.cfg.TemplateOutputDirectory, "model", fmt.Sprintf("%s%s%s%s", tableFilenamePrefix, *table.TableName, tableFilenameSuffix, tableFilenameGo))
		if err = s.writeFile(modelSchemaContentBuffer, modelSchemaContentFilename); err != nil {
			return err
		}

	}

	// model_schema.go
	{
		schema := &TmplTableModelSchema{
			Version:       s.Version,
			PrefixPackage: s.cfg.ImportPrefixPackageName,
		}
		length := len(tables)
		defines := make([]string, 0, length)
		assigns := make([]string, 0, length)
		storage := make([]string, 0, length)
		slice := make([]string, 0, length)
		schemaName := "SCHEMA"
		for _, table := range tables {
			namePascal := table.pascal()
			defines = append(defines, fmt.Sprintf("%s *%s%s", namePascal, schemaName, namePascal))
			assigns = append(assigns, fmt.Sprintf("%s: New%s%s(way),", namePascal, schemaName, namePascal))
			storage = append(storage, fmt.Sprintf("tmp.%s.Table(): tmp.%s,", namePascal, namePascal))
			slice = append(slice, fmt.Sprintf("tmp.%s.Table(),", namePascal))
		}
		schema.MapListDefine = strings.Join(defines, "\n\t")
		schema.MapListAssign = strings.Join(assigns, "\n\t\t")
		schema.MapListStorage = strings.Join(storage, "\n\t\t")
		schema.MapListSlice = strings.Join(slice, "\n\t\t")
		if err := tmpModelSchema.Execute(modelSchemaBuffer, schema); err != nil {
			return err
		}
		if err := s.writeFile(modelSchemaBuffer, modelSchemaFilename); err != nil {
			return err
		}
	}

	// model_schema_custom.go
	{
		if err := tmpModelSchemaCustom.Execute(modelSchemaCustomBuffer, s); err != nil {
			return err
		}

		// if _, err := os.Stat(modelSchemaCustomFilename); err != nil {
		if err := s.writeFile(modelSchemaCustomBuffer, modelSchemaCustomFilename); err != nil {
			return err
		}
		// }
	}

	// table_create.sql
	{
		if err := s.writeFile(modelTableCreateBuffer, modelTableCreateFilename); err != nil {
			return err
		}
	}

	return nil
}

func NewTemplate(name string, content []byte) *template.Template {
	return template.Must(template.New(name).Delims(templateLeft, templateRight).Parse(*(*string)(unsafe.Pointer(&content))))
}

func pathJoin(items ...string) string {
	return filepath.Join(items...)
}

// SchemaTable 数据库表结构
type SchemaTable struct {
	app              *App            `db:"-"`
	TableSchema      *string         `db:"table_schema"`  // 数据库名
	TableName        *string         `db:"table_name"`    // 表名
	TableComment     *string         `db:"table_comment"` // 表注释
	TableFieldSerial string          `db:"-"`             // 表自动递增字段
	Column           []*SchemaColumn `db:"-"`             // 表中的所有字段
	DDL              string          `db:"-"`             // 表定义语句
}

func (s *SchemaTable) pascal() string {
	return utils.Pascal(*s.TableName)
}

func (s *SchemaTable) pascalFirstLower() string {
	return utils.PascalFirstLower(*s.TableName)
}

func (s *SchemaTable) comment() string {
	if s.TableComment == nil {
		return ""
	}
	return *s.TableComment
}

func (s *SchemaTable) newTmplTableModel() (*TmplTableModel, error) {
	tmp := &TmplTableModel{
		table:                s,
		Version:              s.app.Version,
		OriginName:           *s.TableName,
		OriginNamePascal:     s.pascal(),
		OriginNameWithPrefix: *s.TableName,
		OriginNameCamel:      s.pascalFirstLower(),
		Comment:              *s.TableName,
	}
	if s.app.cfg.UsingTableSchemaName && s.app.cfg.TableSchemaName != "" {
		tmp.OriginNameWithPrefix = fmt.Sprintf("%s.%s", s.app.cfg.TableSchemaName, *s.TableName)
	}
	if s.TableComment != nil && *s.TableComment != "" {
		tmp.Comment = *s.TableComment
	}
	if err := tmp.prepare(); err != nil {
		return nil, err
	}
	return tmp, nil
}

// SchemaColumn 表字段结构
type SchemaColumn struct {
	table                  *SchemaTable `db:"-"`
	TableSchema            *string      `db:"table_schema"`             // 数据库名
	TableName              *string      `db:"table_name"`               // 表名
	ColumnName             *string      `db:"column_name"`              // 列名
	OrdinalPosition        *int         `db:"ordinal_position"`         // 列序号
	ColumnDefault          *string      `db:"column_default"`           // 列默认值
	IsNullable             *string      `db:"is_nullable"`              // 是否允许列值为null
	DataType               *string      `db:"data_type"`                // 列数据类型
	CharacterMaximumLength *int         `db:"character_maximum_length"` // 字符串最大长度
	CharacterOctetLength   *int         `db:"character_octet_length"`   // 文本字符串字节最大长度
	NumericPrecision       *int         `db:"numeric_precision"`        // 整数最长长度|小数(整数+小数)合计长度
	NumericScale           *int         `db:"numeric_scale"`            // 小数精度长度
	CharacterSetName       *string      `db:"character_set_name"`       // 字符集名称
	CollationName          *string      `db:"collation_name"`           // 校对集名称
	ColumnComment          *string      `db:"column_comment"`           // 列注释
	ColumnType             *string      `db:"column_type"`              // 列类型
	ColumnKey              *string      `db:"column_key"`               // 列索引 '', 'PRI', 'UNI', 'MUL'
	Extra                  *string      `db:"extra"`                    // 列额外属性 auto_increment
}

func (s *SchemaColumn) databaseTypeToGoType() (types string) {
	nullable := true
	if s.IsNullable != nil && strings.ToLower(*s.IsNullable) == "no" {
		nullable = false
	}
	datatype := ""
	if s.DataType != nil {
		datatype = strings.ToLower(*s.DataType)
	}
	switch datatype {
	case "tinyint":
		types = "int8"
	case "smallint", "smallserial":
		types = "int16"
	case "integer", "serial", "int":
		types = "int"
	case "bigint", "bigserial":
		types = "int64"
	case "decimal", "numeric", "real", "double precision", "double", "float":
		types = "float64"
	case "char", "character", "character varying", "text", "varchar", "enum", "mediumtext", "longtext":
		types = "string"
	case "bool", "boolean":
		types = "bool"
	default:
		types = "string"
	}
	if nullable {
		types = "*" + types
	}
	return
}

func (s *SchemaColumn) pascal() string {
	return utils.Pascal(*s.ColumnName)
}

func (s *SchemaColumn) pascalFirstLower() string {
	return utils.PascalFirstLower(*s.ColumnName)
}

func (s *SchemaColumn) upper() string {
	return utils.Upper(*s.ColumnName)
}

func (s *SchemaColumn) comment() string {
	if s.ColumnComment == nil {
		return ""
	}
	return *s.ColumnComment
}
