package dbs

import (
	"bytes"
	"database/sql"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unsafe"

	"github.com/google/wire"

	"github.com/cd365/hey"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

const (
	byteUnderline byte = '_'
	byte34             = '"'
	byte96             = '`'
	templateLeft       = "{{{"
	templateRight      = "}}}"
)

type TypeDriver string

const (
	DriverMysql    TypeDriver = "mysql"
	DriverPostgres TypeDriver = "postgres"
)

type Caller interface {
	Queries() error
	Tables() []*SysTable
}

type Param struct {
	Version                 string   // template version
	Driver                  string   // driver name
	DataSourceName          string   // data source name
	DatabaseSchemaName      string   // 数据库模式名称
	ImportModelPackageName  string   // model包全名
	UsingDatabaseSchemaName bool     // 是否在表名之前使用模式前缀名称 mysql:数据库名 pgsql:模式名
	FieldsAutoIncrement     string   // 表自动递增字段
	FieldsListCreatedAt     string   // 创建时间戳字段列表, 多个","隔开
	FieldsListUpdatedAt     string   // 更新时间戳字段列表, 多个","隔开
	FieldsListDeletedAt     string   // 删除时间戳字段列表, 多个","隔开
	Discern                 string   // 辨别标识符符号 mysql:` pgsql:"
	OutputDirectory         string   // 输出路径
	way                     *hey.Way // 数据库连接对象
	caller                  Caller   // Caller
}

var (
	//go:embed tmpl/wire.tmpl
	tmplWire []byte

	//go:embed tmpl/model_schema.tmpl
	tmplModelSchema []byte

	//go:embed tmpl/model_schema_content.tmpl
	tmplModelSchemaContent []byte

	//go:embed tmpl/data_schema.tmpl
	tmplDataSchema []byte

	//go:embed tmpl/data_schema_content.tmpl
	tmplDataSchemaContent []byte
)

var (
	_ = wire.NewSet(wire.Value([]string(nil)))
)

func (s *Param) TypeDriver() TypeDriver {
	return TypeDriver(strings.ToLower(s.Driver))
}

func (s *Param) initialize() error {
	s.Driver = strings.TrimSpace(s.Driver)
	db, err := sql.Open(s.Driver, s.DataSourceName)
	if err != nil {
		return err
	}
	switch s.TypeDriver() {
	case DriverMysql:
		s.Discern = "`"
		s.way = hey.NewWay(db)
		s.caller = NewMysql(s, s.way)

		if s.DatabaseSchemaName == "" {
			start := strings.Index(s.DataSourceName, "/")
			if start > -1 {
				end := strings.Index(s.DataSourceName, "?")
				if end > -1 {
					s.DatabaseSchemaName = s.DataSourceName[start+1 : end]
				} else {
					s.DatabaseSchemaName = s.DataSourceName[start+1:]
				}
			}
			s.DatabaseSchemaName = strings.TrimSpace(s.DatabaseSchemaName)
		}
	case DriverPostgres:
		s.Discern = "\""
		s.way = hey.NewWay(db, hey.WithPrepare(hey.DefaultPgsql.Prepare))
		s.caller = NewPgsql(s, s.way)

		if s.DatabaseSchemaName == "" {
			s.DatabaseSchemaName = "public"
		}
	default:
		err = fmt.Errorf("unsupported driver name: %s", s.Driver)
	}
	return nil
}

func NewTemplate(name string, content []byte) *template.Template {
	return template.Must(template.New(name).Delims(templateLeft, templateRight).Parse(*(*string)(unsafe.Pointer(&content))))
}

func pascal(name string) string {
	length := len(name)
	tmp := make([]byte, 0, length)
	next2upper := true
	for i := 0; i < length; i++ {
		if name[i] == byteUnderline {
			next2upper = true
			continue
		}
		if next2upper && name[i] >= 'a' && name[i] <= 'z' {
			tmp = append(tmp, name[i]-32)
		} else {
			tmp = append(tmp, name[i])
		}
		next2upper = false
	}
	return string(tmp[:])
}

func pathJoin(items ...string) string {
	return filepath.Join(items...)
}

// CopyReaderToFile copy io.Reader to file
func CopyReaderToFile(writer io.Reader, filename string) error {
	dir := filepath.Dir(filename)
	if _, err := os.Stat(dir); err != nil {
		if err = os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	if _, err := os.Stat(filename); err == nil {
		if err = os.Remove(filename); err != nil {
			return err
		}
	}
	fil, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() { _ = fil.Close() }()
	if _, err = io.Copy(fil, writer); err != nil {
		return err
	}
	return nil
}

// SysTable 数据库表结构
type SysTable struct {
	way          *hey.Way     `db:"-"`
	TableSchema  *string      `db:"table_schema"`  // 数据库名
	TableName    *string      `db:"table_name"`    // 表名
	TableComment *string      `db:"table_comment"` // 表注释
	Column       []*SysColumn `db:"-"`             // 表中的所有字段
}

func (s *SysTable) pascal() string {
	if s.TableName == nil {
		return ""
	}
	return pascal(*s.TableName)
}

func (s *SysTable) comment() string {
	if s.TableComment == nil {
		return ""
	}
	return *s.TableComment
}

// SysColumn 表字段结构
type SysColumn struct {
	TableSchema            *string `db:"table_schema"`             // 数据库名
	TableName              *string `db:"table_name"`               // 表名
	ColumnName             *string `db:"column_name"`              // 列名
	OrdinalPosition        *int    `db:"ordinal_position"`         // 列序号
	ColumnDefault          *string `db:"column_default"`           // 列默认值
	IsNullable             *string `db:"is_nullable"`              // 是否允许列值为null
	DataType               *string `db:"data_type"`                // 列数据类型
	CharacterMaximumLength *int    `db:"character_maximum_length"` // 字符串最大长度
	CharacterOctetLength   *int    `db:"character_octet_length"`   // 文本字符串字节最大长度
	NumericPrecision       *int    `db:"numeric_precision"`        // 整数最长长度|小数(整数+小数)合计长度
	NumericScale           *int    `db:"numeric_scale"`            // 小数精度长度
	CharacterSetName       *string `db:"character_set_name"`       // 字符集名称
	CollationName          *string `db:"collation_name"`           // 校对集名称
	ColumnComment          *string `db:"column_comment"`           // 列注释
	ColumnType             *string `db:"column_type"`              // 列类型
	ColumnKey              *string `db:"column_key"`               // 列索引 '', 'PRI', 'UNI', 'MUL'
	Extra                  *string `db:"extra"`                    // 列额外属性 auto_increment
}

func (s *SysColumn) databaseTypeToGoType() (types string) {
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
	if nullable && types != "" {
		types = "*" + types
	}
	return
}

func (s *SysColumn) pascal() string {
	if s.ColumnName == nil {
		return ""
	}
	return pascal(*s.ColumnName)
}

func (s *SysColumn) comment() string {
	if s.ColumnComment == nil {
		return ""
	}
	return *s.ColumnComment
}

func bufferTable(fn func(table *SysTable) string, tables ...*SysTable) *bytes.Buffer {
	buffer := bytes.NewBuffer(nil)
	for _, table := range tables {
		buffer.WriteString(fn(table))
	}
	return buffer
}

func buildWire(pkg string, tables []*SysTable) *Wires {
	fn := func(table *SysTable) string {
		tmp := fmt.Sprintf("\n\tNew%s,", table.pascal())
		comment := table.comment()
		if comment != "" {
			tmp = fmt.Sprintf("%s // %s", tmp, comment)
		}
		return tmp
	}
	buffer := bufferTable(fn, tables...)
	buffer.WriteString("\n")
	return &Wires{
		DefinePackageName: pkg,
		WireContent:       buffer.String(),
	}
}

// createModel create model.go
func (s *Param) createModel() error {
	temp := NewTemplate("tmpl_wire", tmplWire)
	buf := bytes.NewBuffer(nil)
	pkg := "model"
	data := buildWire(pkg, s.caller.Tables())
	data.Version = s.Version
	if err := temp.Execute(buf, data); err != nil {
		return err
	}
	return CopyReaderToFile(buf, pathJoin(s.OutputDirectory, pkg, fmt.Sprintf("%s.go", pkg)))
}

// createModelSchema create model_schema.go
func (s *Param) createModelSchema() error {
	tmpModelSchema := NewTemplate("tmpl_model_schema", tmplModelSchema)
	tmpModelSchemaContent := NewTemplate("tmpl_model_schema_content", tmplModelSchemaContent)
	buf := bytes.NewBuffer(nil)
	for _, table := range s.caller.Tables() {
		data := s.createModelSchemaTable(table)
		data.Version = s.Version
		if err := tmpModelSchemaContent.Execute(buf, data); err != nil {
			return err
		}
	}

	content := bytes.NewBuffer(nil)
	data := &ModelSchemas{
		Version: s.Version,
		Content: buf.String(),
	}
	if err := tmpModelSchema.Execute(content, data); err != nil {
		return err
	}

	return CopyReaderToFile(content, pathJoin(s.OutputDirectory, "model", "model_schema.go"))
}

// createData create data.go
func (s *Param) createData() (err error) {
	temp := NewTemplate("tmpl_wire", tmplWire)
	buf := bytes.NewBuffer(nil)
	pkg := "data"
	data := buildWire(pkg, s.caller.Tables())
	data.Version = s.Version
	if err = temp.Execute(buf, data); err != nil {
		return
	}
	err = CopyReaderToFile(buf, pathJoin(s.OutputDirectory, pkg, fmt.Sprintf("%s.go", pkg)))
	return
}

// createDataSchema create data_schema.go
func (s *Param) createDataSchema() error {
	tmpDataSchema := NewTemplate("data_schema", tmplDataSchema)
	tmpDataSchemaContent := NewTemplate("data_schema_content", tmplDataSchemaContent)

	buf := bytes.NewBuffer(nil)
	for _, table := range s.caller.Tables() {
		data := s.createDataSchemaTable(table)
		data.Version = s.Version
		if err := tmpDataSchemaContent.Execute(buf, data); err != nil {
			return err
		}
	}

	outer := bytes.NewBuffer(nil)
	data := &DataSchemas{
		Version:                    s.Version,
		ImportModelPackage:         s.ImportModelPackageName,
		DataAllTablesSchemaContent: buf.String(),
	}
	if err := tmpDataSchema.Execute(outer, data); err != nil {
		return err
	}

	return CopyReaderToFile(outer, pathJoin(s.OutputDirectory, "data", "data_schema.go"))
}

// BuildAll build all template
func (s *Param) BuildAll() error {
	err := s.initialize()
	if err != nil {
		return err
	}
	// query
	if err = s.caller.Queries(); err != nil {
		return err
	}
	// model
	if err = s.createModel(); err != nil {
		return err
	}
	if err = s.createModelSchema(); err != nil {
		return err
	}
	// data
	if err = s.createData(); err != nil {
		return err
	}
	if err = s.createDataSchema(); err != nil {
		return err
	}
	return nil
}

type DataSchemas struct {
	Version                    string // template version
	ImportModelPackage         string
	DataAllTablesSchemaContent string
}

type Wires struct {
	Version           string // template version
	DefinePackageName string // package name
	WireContent       string // wire content
}

type ModelSchemas struct {
	Version string // template version
	Content string
}

type CreateDataSchemaTable struct {
	Version         string // template version
	TableNamePascal string // 表名(帕斯卡命名) ==> AccountMoneyLog
	TableName       string // 表名(数据库原始命名) ==> account_money_log
	TableNameSchema string // 表名(带数据库模式前缀名) ==> public.account_money_log
	TableComment    string // 表注释 ==> 账户日志明细表
}

// createDataSchemaTable create data schema table
func (s *Param) createDataSchemaTable(table *SysTable) (model *CreateDataSchemaTable) {
	model = &CreateDataSchemaTable{
		TableNamePascal: table.pascal(),
		TableName:       *table.TableName,
		TableNameSchema: "",
		TableComment:    "",
	}
	if table.TableComment != nil {
		model.TableComment = *table.TableComment
	}
	if table.TableSchema != nil {
		model.TableNameSchema = fmt.Sprintf("%s.%s", *table.TableSchema, model.TableName)
	}
	return
}

type CreateModelSchemaTable struct {
	Version             string // template version
	TableNamePascal     string // 表名(帕斯卡命名) ==> AccountMoneyLog
	TableName           string // 表名(数据库原始命名) ==> account_money_log
	TableNameWithSchema string // 表名(带数据库模式前缀名) ==> public.account_money_log
	TableComment        string // 表注释 ==> 账户日志明细表

	TableStructColumn                   []string // 表结构体字段定义 ==> Name string `json:"name" db:"name"` // 名称
	TableStructColumnHey                []string // 表结构体字段关系定义 ==> Name string // name 名称
	TableStructColumnHeyFieldSlice      string   // NewHey.Field ==> // []string{"id", "name"}
	TableStructColumnHeyFieldSliceValue string   // NewHey.FieldStr ==> // `"id", "name"` || "`id`, `name`"
	TableStructColumnReq                []string // 表结构体字段定义 ==> Name *string `json:"name" db:"name"` // 名称

	TableStructColumnHeyValues          []string // NewHey.Attribute ==> Name:"name", // 名称
	TableStructColumnHeyValuesAccess    string   // NewHey.Access ==> Access:[]string{}, // 访问字段列表
	TableStructColumnHeyValuesAccessMap string   // NewHey.AccessMap ==> Access:map[string]struct{}, // 访问字段列表

	TableColumnAutoIncr  string // 结构体字段方法 ColumnAutoIncr
	TableColumnCreatedAt string // 结构体字段方法 ColumnCreatedAt
	TableColumnUpdatedAt string // 结构体字段方法 ColumnUpdatedAt
	TableColumnDeletedAt string // 结构体字段方法 ColumnDeletedAt
}

// createModelSchemaTable create model schema table
func (s *Param) createModelSchemaTable(table *SysTable) (model *CreateModelSchemaTable) {
	model = &CreateModelSchemaTable{
		TableNamePascal: table.pascal(),
		TableName:       *table.TableName,
	}
	if table.TableComment != nil {
		model.TableComment = *table.TableComment
	}
	if table.TableSchema != nil {
		if s.UsingDatabaseSchemaName {
			model.TableNameWithSchema = fmt.Sprintf("%s.%s", *table.TableSchema, model.TableName)
		} else {
			model.TableNameWithSchema = model.TableName
		}
	}

	// struct define
	for i, c := range table.Column {
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
		model.TableStructColumn = append(model.TableStructColumn, tmp)
	}

	// hey
	for i, c := range table.Column {
		tmp := fmt.Sprintf("\t%s string", c.pascal())
		comment := c.comment()
		if comment != "" {
			tmp = fmt.Sprintf("%s // %s", tmp, comment)
		}
		if i != 0 {
			tmp = fmt.Sprintf("\n%s", tmp)
		}
		model.TableStructColumnHey = append(model.TableStructColumnHey, tmp)
	}

	// column list
	{
		lengthColumn := len(table.Column)
		field := make([]string, 0, lengthColumn)
		fieldAccess := make([]string, 0, lengthColumn)
		for i, c := range table.Column {
			field = append(field, fmt.Sprintf("\"%s\"", *c.ColumnName))
			fieldPascalName := c.pascal()
			fieldAccessTmp := fmt.Sprintf("s.%s,", fieldPascalName)
			if c.ColumnComment != nil {
				fieldAccessTmp = fmt.Sprintf("%s // %s", fieldAccessTmp, *c.ColumnComment)
			}
			fieldAccess = append(fieldAccess, fieldAccessTmp)
			tmp := fmt.Sprintf("\t\t%s:\"%s\",", fieldPascalName, *c.ColumnName)
			comment := c.comment()
			if comment != "" {
				tmp = fmt.Sprintf("%s // %s", tmp, comment)
			}
			if i != 0 {
				tmp = fmt.Sprintf("\n%s", tmp)
			}
			model.TableStructColumnHeyValues = append(model.TableStructColumnHeyValues, tmp)
		}

		{
			s96 := string(byte96) // `
			s34 := `"`            // "
			model.TableStructColumnHeyFieldSlice = strings.Join(field, ", ")
			switch s.TypeDriver() {
			case DriverMysql:
				model.TableStructColumnHeyFieldSlice = strings.ReplaceAll(model.TableStructColumnHeyFieldSlice, s34, s96)
			}
			if strings.Index(model.TableStructColumnHeyFieldSlice, s96) >= 0 {
				model.TableStructColumnHeyFieldSliceValue = hey.ConcatString(s34, model.TableStructColumnHeyFieldSlice, s34)
			} else {
				model.TableStructColumnHeyFieldSliceValue = hey.ConcatString(s96, model.TableStructColumnHeyFieldSlice, s96)
			}
		}

		model.TableStructColumnHeyValuesAccess = fmt.Sprintf("[]string{\n\t\t%s\n\t}", strings.Join(fieldAccess, "\n\t\t"))

		fieldAccessMap := fieldAccess[:]
		for k, v := range fieldAccessMap {
			fieldAccessMap[k] = strings.Replace(v, ",", ":{},", 1)
		}
		model.TableStructColumnHeyValuesAccessMap = fmt.Sprintf("map[string]struct{}{\n\t\t%s\n\t}", strings.Join(fieldAccessMap, "\n\t\t"))
	}

	// ignore columns, for insert and update
	var ignore []string

	// table special fields
	{
		// auto increment field or timestamp field
		cm := make(map[string]*SysColumn)
		for _, v := range table.Column {
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
		autoIncrement := fc(s.FieldsAutoIncrement)                  // auto increment column
		created := fc(strings.Split(s.FieldsListCreatedAt, ",")...) // created_at columns
		updated := fc(strings.Split(s.FieldsListUpdatedAt, ",")...) // updated_at columns
		deleted := fc(strings.Split(s.FieldsListDeletedAt, ",")...) // deleted_at columns

		ignore = append(ignore, autoIncrement[:]...)
		ignore = append(ignore, created[:]...)
		ignore = append(ignore, updated[:]...)
		ignore = append(ignore, deleted[:]...)

		if len(autoIncrement) > 0 && autoIncrement[0] != "" {
			model.TableColumnAutoIncr = fmt.Sprintf("[]string{ s.%s }", pascal(autoIncrement[0]))
		} else {
			model.TableColumnAutoIncr = "nil"
		}
		cs := func(cols ...string) string {
			length := len(cols)
			if length == 0 {
				return "nil"
			}
			for i := 0; i < length; i++ {
				cols[i] = fmt.Sprintf("s.%s", pascal(cols[i]))
			}
			return fmt.Sprintf("[]string{ %s }", strings.Join(cols, ", "))
		}
		model.TableColumnCreatedAt = cs(created...)
		model.TableColumnUpdatedAt = cs(updated...)
		model.TableColumnDeletedAt = cs(deleted...)
	}

	ignoreMap := make(map[string]struct{})
	for _, v := range ignore {
		ignoreMap[v] = struct{}{}
	}

	// req
	write := false
	for _, c := range table.Column {
		if _, ok := ignoreMap[*c.ColumnName]; ok {
			continue // ignore columns like id, created_at, updated_at, deleted_at
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

		comment := c.comment()
		if comment != "" {
			tmp = fmt.Sprintf("%s // %s", tmp, comment)
		}
		if write {
			tmp = fmt.Sprintf("\n%s", tmp)
		} else {
			write = true
		}
		model.TableStructColumnReq = append(model.TableStructColumnReq, tmp)
	}

	return
}
