package dbs

import (
	"bytes"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
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
	ShowCreateTable(table *SysTable) (string, error)
}

type Param struct {
	Version                 string   // template version
	Driver                  string   // driver name
	DataSourceName          string   // data source name
	DatabaseSchemaName      string   // 数据库模式名称
	ImportModelPackageName  string   // model包全名
	BizCommon               bool     // biz common.go
	BizCommonContent        string   // biz common.go table1.field1,table2.field2,table3.field3...
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

	//go:embed tmpl/biz_schema.tmpl
	tmplBizSchema []byte

	//go:embed tmpl/biz_schema_content.tmpl
	tmplBizSchemaContent []byte

	//go:embed tmpl/biz_common.tmpl
	tmplBizCommon []byte

	//go:embed tmpl/biz_common_content.tmpl
	tmplBizCommonContent []byte

	//go:embed pgsql_func_create.sql
	pgsqlFuncCreate string

	//go:embed pgsql_func_drop.sql
	pgsqlFuncDrop string
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
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(2)
	db.SetConnMaxIdleTime(time.Minute * 3)
	db.SetConnMaxLifetime(time.Minute * 3)
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

func createFile(filename string) (*os.File, error) {
	dir := filepath.Dir(filename)
	if _, err := os.Stat(dir); err != nil {
		if err = os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(filename); err == nil {
		if err = os.Remove(filename); err != nil {
			return nil, err
		}
	}
	return os.Create(filename)
}

// SysTable 数据库表结构
type SysTable struct {
	way                *hey.Way     `db:"-"`
	TableSchema        *string      `db:"table_schema"`  // 数据库名
	TableName          *string      `db:"table_name"`    // 表名
	TableComment       *string      `db:"table_comment"` // 表注释
	TableAutoIncrement string       `db:"-"`             // 表自动递增字段名称
	Column             []*SysColumn `db:"-"`             // 表中的所有字段
}

func (s *SysTable) pascal() string {
	if s.TableName == nil {
		return ""
	}
	return pascal(*s.TableName)
}

func (s *SysTable) pascalSmall() string {
	name := s.pascal()
	if len(name) == 0 {
		return ""
	}
	return strings.ToLower(name[0:1]) + name[1:]
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

type TemplateData struct {
	// version
	TemplateVersion string // template version

	// biz
	BizImportDataPackageName  string // biz.go data import model package name
	BizAllTablesSchemaContent string // biz.go data all tables schema content

	// data
	DataImportModelPackageName string // data.go data import model package name
	DataAllTablesSchemaContent string // data.go data all tables schema content
	DataMapListDefine          string // data_schema.go tables define
	DataMapListParams          string // data_schema.go tables params
	DataMapListAssign          string // data_schema.go tables assign
	DataMapListStorage         string // data_schema.go tables storage
	DataMapListSlice           string // data_schema.go tables slice

	// wire
	WireDefinePackageName string // wire.go define package name
	WireContent           string // wire.go content

	// table
	TableNamePascal      string // 表名(帕斯卡命名)
	TableName            string // 数据库原始表名
	TableNameSchema      string // 表模式名称或者数据库名称(表前缀名)
	TableComment         string // 表注释
	TableNameWithSchema  string // 表名(带前缀)
	TableNameSmallPascal string // 表名(小帕斯卡命名)

	// model
	ModelAllTablesSchemaContent         string   // model.go model all tables schema content
	TableStructColumn                   []string // 表结构体字段定义 ==> Name string `json:"name" db:"name"` // 名称
	TableStructColumnHey                []string // 表结构体字段关系定义 ==> Name string // name 名称
	TableStructColumnHeyFieldSlice      string   // NewHey.Field ==> // []string{"id", "name"}
	TableStructColumnHeyFieldSliceValue string   // NewHey.FieldStr ==> // `"id", "name"` || "`id`, `name`"
	TableStructColumnReq                []string // 表结构体字段定义 ==> Name *string `json:"name" db:"name"` // 名称
	TableStructColumnUpdate             string   // 表结构体字段更新 ==> if s.Id != t.Id { tmp["id"] = t.Id }

	TableStructColumnHeyValues          []string // NewHey.Attribute ==> Name:"name", // 名称
	TableStructColumnHeyValuesAccess    string   // NewHey.Access ==> Access:[]string{}, // 访问字段列表
	TableStructColumnHeyValuesAccessMap string   // NewHey.AccessMap ==> Access:map[string]struct{}, // 访问字段列表

	TableColumnAutoIncr  string // 结构体字段方法 ColumnAutoIncr
	TableColumnCreatedAt string // 结构体字段方法 ColumnCreatedAt
	TableColumnUpdatedAt string // 结构体字段方法 ColumnUpdatedAt
	TableColumnDeletedAt string // 结构体字段方法 ColumnDeletedAt

	// ddl
	TableDdl string // table ddl
}

func bufferTable(fn func(i int, table *SysTable) string, tables ...*SysTable) *bytes.Buffer {
	buffer := bytes.NewBuffer(nil)
	for index, table := range tables {
		buffer.WriteString(fn(index, table))
	}
	return buffer
}

func buildWire(pkg string, tables []*SysTable) *TemplateData {
	fn := func(i int, table *SysTable) string {
		tmp := fmt.Sprintf("New%s,", table.pascal())
		if i > 0 {
			tmp = fmt.Sprintf("\n\t%s", tmp)
		}
		comment := table.comment()
		if comment != "" {
			tmp = fmt.Sprintf("%s // %s", tmp, comment)
		}
		return tmp
	}
	buffer := bufferTable(fn, tables...)
	if pkg == "data" {
		buffer.WriteString(fmt.Sprintf("\n\tNewTables, // all instances"))
	}
	return &TemplateData{
		WireDefinePackageName: pkg,
		WireContent:           buffer.String(),
	}
}

// createModel create model.go
func (s *Param) createModel() (err error) {
	temp := NewTemplate("tmpl_wire", tmplWire)
	pkg := "model"
	filename := pathJoin(s.OutputDirectory, pkg, fmt.Sprintf("%s.go", pkg))
	var fil *os.File
	if fil, err = createFile(filename); err != nil {
		return
	}
	defer func() {
		_ = fil.Close()
		if err != nil {
			_ = os.Remove(filename)
		}
	}()
	data := buildWire(pkg, s.caller.Tables())
	data.TemplateVersion = s.Version
	if err = temp.Execute(fil, data); err != nil {
		return
	}
	return
}

// createModelSchema create model_schema.go
func (s *Param) createModelSchema() (err error) {
	tmpModelSchema := NewTemplate("tmpl_model_schema", tmplModelSchema)
	tmpModelSchemaContent := NewTemplate("tmpl_model_schema_content", tmplModelSchemaContent)

	modelSchemaFilename := pathJoin(s.OutputDirectory, "model", "model_schema.go")
	var modelSchemaContent *os.File
	if modelSchemaContent, err = createFile(modelSchemaFilename); err != nil {
		return
	}
	defer func() {
		_ = modelSchemaContent.Close()
		if err != nil {
			_ = os.Remove(modelSchemaFilename)
		}
	}()

	modelSchemaContentBuffer := bytes.NewBuffer(nil)

	modelTableCreateFilename := pathJoin(s.OutputDirectory, "model", "table_create.sql")
	var modelTableCreateContent *os.File
	if modelTableCreateContent, err = createFile(modelTableCreateFilename); err != nil {
		return
	}
	defer func() {
		_ = modelTableCreateContent.Close()
		if err != nil {
			_ = os.Remove(modelTableCreateFilename)
		}
	}()

	create := ""
	for _, table := range s.caller.Tables() {
		{

			// for table ddl
			create, err = s.caller.ShowCreateTable(table)
			if err != nil {
				return
			}
			for strings.HasSuffix(create, "\n") {
				create = strings.TrimSuffix(create, "\n")
			}
			if _, err = modelTableCreateContent.WriteString(create); err != nil {
				return
			}
			if !strings.HasSuffix(create, ";") {
				if _, err = modelTableCreateContent.WriteString(";"); err != nil {
					return
				}
			}
			if _, err = modelTableCreateContent.WriteString("\n"); err != nil {
				return
			}
		}
		data := s.createModelSchemaTable(table)
		data.TemplateVersion = s.Version
		if err = tmpModelSchemaContent.Execute(modelSchemaContentBuffer, data); err != nil {
			return
		}
	}

	data := &TemplateData{
		TemplateVersion:             s.Version,
		ModelAllTablesSchemaContent: modelSchemaContentBuffer.String(),
	}
	if err = tmpModelSchema.Execute(modelSchemaContent, data); err != nil {
		return
	}
	return
}

// createData create data.go
func (s *Param) createData() (err error) {
	temp := NewTemplate("tmpl_wire", tmplWire)
	pkg := "data"
	filename := pathJoin(s.OutputDirectory, pkg, fmt.Sprintf("%s.go", pkg))
	var fil *os.File
	if fil, err = createFile(filename); err != nil {
		return
	}
	defer func() {
		_ = fil.Close()
		if err != nil {
			_ = os.Remove(filename)
		}
	}()
	data := buildWire(pkg, s.caller.Tables())
	data.TemplateVersion = s.Version
	if err = temp.Execute(fil, data); err != nil {
		return
	}
	return
}

// createDataSchema create data_schema.go
func (s *Param) createDataSchema() (err error) {
	tmpDataSchema := NewTemplate("data_schema", tmplDataSchema)
	tmpDataSchemaContent := NewTemplate("data_schema_content", tmplDataSchemaContent)

	contents := bytes.NewBuffer(nil)
	tables := s.caller.Tables()
	for _, table := range tables {
		data := s.createDataSchemaTable(table)
		data.TemplateVersion = s.Version
		if err = tmpDataSchemaContent.Execute(contents, data); err != nil {
			return err
		}
	}

	filename := pathJoin(s.OutputDirectory, "data", "data_schema.go")
	var fil *os.File
	if fil, err = createFile(filename); err != nil {
		return
	}
	defer func() {
		_ = fil.Close()
		if err != nil {
			_ = os.Remove(filename)
		}
	}()
	data := &TemplateData{
		TemplateVersion:            s.Version,
		DataImportModelPackageName: s.ImportModelPackageName,
		DataAllTablesSchemaContent: contents.String(),
	}
	length := len(tables)
	defines := make([]string, 0, length)
	params := make([]string, 0, length)
	assigns := make([]string, 0, length)
	storage := make([]string, 0, length)
	slice := make([]string, 0, length)
	for _, table := range tables {
		namePascal := table.pascal()
		namePascalSmall := table.pascalSmall()
		defines = append(defines, fmt.Sprintf("%s *%s", namePascal, namePascal))
		params = append(params, fmt.Sprintf("%s *%s,", namePascalSmall, namePascal))
		assigns = append(assigns, fmt.Sprintf("%s: %s,", namePascal, namePascalSmall))
		storage = append(storage, fmt.Sprintf("%s.Table(): %s,", namePascalSmall, namePascalSmall))
		slice = append(slice, fmt.Sprintf("%s.Table(),", namePascalSmall))
	}
	data.DataMapListDefine = strings.Join(defines, "\n\t")
	data.DataMapListParams = strings.Join(params, "\n\t")
	data.DataMapListAssign = strings.Join(assigns, "\n\t\t")
	data.DataMapListStorage = strings.Join(storage, "\n\t\t")
	data.DataMapListSlice = strings.Join(slice, "\n\t\t")
	if err = tmpDataSchema.Execute(fil, data); err != nil {
		return
	}
	return
}

// createBiz create biz.tmpl
func (s *Param) createBiz() (err error) {
	temp := NewTemplate("tmpl_wire", tmplWire)
	pkg := "biz"
	filename := pathJoin(s.OutputDirectory, pkg, fmt.Sprintf("%s.tmpl", pkg))
	var fil *os.File
	if fil, err = createFile(filename); err != nil {
		return
	}
	defer func() {
		_ = fil.Close()
		if err != nil {
			_ = os.Remove(filename)
		}
	}()
	data := buildWire(pkg, s.caller.Tables())
	data.TemplateVersion = s.Version
	if err = temp.Execute(fil, data); err != nil {
		return
	}
	return
}

// createBizSchema create biz_schema.tmpl
func (s *Param) createBizSchema() (err error) {
	tmpBizSchema := NewTemplate("biz_schema", tmplBizSchema)
	tmpBizSchemaContent := NewTemplate("biz_schema_content", tmplBizSchemaContent)

	buf := bytes.NewBuffer(nil)

	for _, table := range s.caller.Tables() {
		data := s.createBizSchemaTable(table)
		data.TemplateVersion = s.Version
		if err = tmpBizSchemaContent.Execute(buf, data); err != nil {
			return err
		}
	}

	filename := pathJoin(s.OutputDirectory, "biz", "biz_schema.tmpl")
	var fil *os.File
	if fil, err = createFile(filename); err != nil {
		return
	}
	defer func() {
		_ = fil.Close()
		if err != nil {
			_ = os.Remove(filename)
		}
	}()
	data := &TemplateData{
		TemplateVersion:           s.Version,
		BizImportDataPackageName:  strings.Replace(s.ImportModelPackageName, "model", "data", 1),
		BizAllTablesSchemaContent: buf.String(),
	}
	if err = tmpBizSchema.Execute(fil, data); err != nil {
		return err
	}
	return
}

type BizCommon struct {
	// version
	TemplateVersion string // template version

	ModuleImportPrefix string // import prefix

	MethodsContent string // all table methods content
}

type BizCommonContent struct {
	TableNamePascal                   string
	TableAutoIncrementFieldName       string
	TableAutoIncrementFieldNamePascal string
}

// createBizCommon create biz/common.go
func (s *Param) createBizCommon() (err error) {
	if !s.BizCommon {
		return
	}
	temp := NewTemplate("tmpl_biz_common", tmplBizCommon)

	pkg := "biz"

	filename := pathJoin(s.OutputDirectory, pkg, "common.go")
	var fil *os.File
	if fil, err = createFile(filename); err != nil {
		return
	}
	defer func() {
		_ = fil.Close()
		if err != nil {
			_ = os.Remove(filename)
		}
	}()

	data := &BizCommon{}
	data.TemplateVersion = s.Version
	data.ModuleImportPrefix = strings.TrimSuffix(s.ImportModelPackageName, "model")

	{
		bcs := strings.Split(s.BizCommonContent, ",")
		bcsMapSlice := make(map[string][]string)
		for _, v := range bcs {
			v = strings.TrimSpace(strings.ReplaceAll(v, " ", ""))
			vv := strings.Split(v, ".")
			if len(vv) != 2 {
				continue
			}
			if _, ok := bcsMapSlice[vv[0]]; !ok {
				bcsMapSlice[vv[0]] = make([]string, 0, 1)
			}
			bcsMapSlice[vv[0]] = append(bcsMapSlice[vv[0]], vv[1])
		}

		content := bytes.NewBuffer(nil)
		writeContentMethod := func(table string, field string) error {
			bcc := &BizCommonContent{
				TableNamePascal:                   pascal(table),
				TableAutoIncrementFieldNamePascal: pascal(field),
			}
			bccTmpl := NewTemplate("tmpl_biz_common_content", tmplBizCommonContent)
			return bccTmpl.Execute(content, bcc)
		}
		for _, table := range s.caller.Tables() {
			if table.TableAutoIncrement != "" {
				if err = writeContentMethod(*table.TableName, table.TableAutoIncrement); err != nil {
					return
				}
			}
			fieldsExists := make(map[string]struct{})
			for _, v := range table.Column {
				fieldsExists[*v.ColumnName] = struct{}{}
			}
			if fields, ok := bcsMapSlice[*table.TableName]; ok {
				for _, f := range fields {
					if _, ok = fieldsExists[f]; !ok {
						continue
					}
					if err = writeContentMethod(*table.TableName, f); err != nil {
						return
					}
				}
			}
		}
		data.MethodsContent = content.String()
	}

	if err = temp.Execute(fil, data); err != nil {
		return
	}
	return
}

// BuildAll build all template
func (s *Param) BuildAll() error {
	err := s.initialize()
	if err != nil {
		return err
	}
	switch TypeDriver(s.Driver) {
	case DriverMysql:
	case DriverPostgres:
		if _, err = s.way.DB().Exec(pgsqlFuncCreate); err != nil {
			return err
		}
		defer func() { _, _ = s.way.DB().Exec(pgsqlFuncDrop) }()
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
	// biz
	if err = s.createBiz(); err != nil {
		return err
	}
	if err = s.createBizSchema(); err != nil {
		return err
	}
	if err = s.createBizCommon(); err != nil {
		return err
	}
	return nil
}

// createBizSchemaTable create biz schema table
func (s *Param) createBizSchemaTable(table *SysTable) (model *TemplateData) {
	model = &TemplateData{
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
	model.TableNameSmallPascal = strings.ToLower(model.TableNamePascal[0:1]) + model.TableNamePascal[1:]
	return
}

// createDataSchemaTable create data schema table
func (s *Param) createDataSchemaTable(table *SysTable) (model *TemplateData) {
	model = &TemplateData{
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

// createModelSchemaTable create model schema table
func (s *Param) createModelSchemaTable(table *SysTable) (model *TemplateData) {
	model = &TemplateData{
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

	columnUpdates := make([]string, 0)

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

		// update column
		o := *c.ColumnName
		p := c.pascal()
		update := fmt.Sprintf(`
	if s.%s != c.%s {
		tmp["%s"] = c.%s
	}`, p, p, o, p)
		columnUpdates = append(columnUpdates, update)
	}

	model.TableStructColumnUpdate = strings.Join(columnUpdates, "")

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
		autoIncrement := fc(s.FieldsAutoIncrement) // auto increment column
		if table.TableAutoIncrement != "" && s.FieldsAutoIncrement != table.TableAutoIncrement {
			autoIncrement = append(autoIncrement, table.TableAutoIncrement)
		}
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
