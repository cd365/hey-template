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
	ShowCreateTable(table *SysTable) error
}

type Writer interface {
	Write() error
}

type Param struct {
	Version                 string   // template version
	Driver                  string   // driver name
	DataSourceName          string   // data source name
	DatabaseSchemaName      string   // 数据库模式名称
	ImportModelPackageName  string   // model包全名
	TableFieldMethodOfData  string   // method of table.field table1.field1,table2.field2,table3.field3...
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

	//go:embed tmpl/data_schema_content_custom.tmpl
	tmplDataSchemaContentCustom []byte

	//go:embed tmpl/biz_schema.tmpl
	tmplBizSchema []byte

	//go:embed tmpl/biz_schema_content.tmpl
	tmplBizSchemaContent []byte

	//go:embed pgsql_func_create.sql
	pgsqlFuncCreate string

	//go:embed pgsql_func_drop.sql
	pgsqlFuncDrop string
)

var _ = wire.NewSet(wire.Value([]string(nil)))

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

func (s *Param) parseCustomTableFields() map[string][]string {
	bcs := strings.Split(s.TableFieldMethodOfData, ",")
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
	return bcsMapSlice
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
	// show create table
	for _, table := range s.caller.Tables() {
		if err = s.caller.ShowCreateTable(table); err != nil {
			return err
		}
	}
	basis := NewBasis(s)
	// build model, data, biz
	writer := make([]Writer, 0, 4)
	writer = append(writer, NewDbModel(basis))
	writer = append(writer, NewDbData(basis))
	writer = append(writer, NewDbBiz(basis))
	for _, w := range writer {
		if err = w.Write(); err != nil {
			return err
		}
	}
	return nil
}

type Basis struct {
	*Param
	tables []*SysTable

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
}

func NewBasis(param *Param) *Basis {
	basis := &Basis{Param: param}
	basis.tables = param.caller.Tables()
	return basis
}

func (s *Basis) buildWire(pkg string) {
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
	buffer := bufferTable(fn, s.tables...)
	if pkg == "data" {
		buffer.WriteString(fmt.Sprintf("\n\tNewTables, // all instances"))
	}
	s.WireDefinePackageName = pkg
	s.WireContent = buffer.String()
}

func (s *Basis) WriteFile(reader io.Reader, filename string) error {
	fil, err := createFile(filename)
	if err != nil {
		return err
	}
	defer func() { _ = fil.Close() }()
	_, err = io.Copy(fil, reader)
	return err
}

type DbModel struct {
	*Basis

	// model-content
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

	// model-ddl
	TableDdl string // table ddl
}

func (s *DbModel) clean() {
	s.TableStructColumn = nil
	s.TableStructColumnHey = nil
	s.TableStructColumnHeyFieldSlice = ""
	s.TableStructColumnHeyFieldSliceValue = ""
	s.TableStructColumnReq = nil
	s.TableStructColumnUpdate = ""
	s.TableStructColumnHeyValues = nil
	s.TableStructColumnHeyValuesAccess = ""
	s.TableStructColumnHeyValuesAccessMap = ""
	s.TableColumnAutoIncr = ""
	s.TableColumnCreatedAt = ""
	s.TableColumnUpdatedAt = ""
	s.TableColumnDeletedAt = ""
	s.TableDdl = ""
}

func NewDbModel(basis *Basis) *DbModel {
	return &DbModel{Basis: basis}
}

func (s *DbModel) createModelSchemaTable(table *SysTable) {
	s.TableNamePascal = table.pascal()
	s.TableName = *table.TableName
	if table.TableComment != nil {
		s.TableComment = *table.TableComment
	}
	if table.TableSchema != nil {
		if s.UsingDatabaseSchemaName {
			s.TableNameWithSchema = fmt.Sprintf("%s.%s", *table.TableSchema, s.TableName)
		} else {
			s.TableNameWithSchema = s.TableName
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
		s.TableStructColumn = append(s.TableStructColumn, tmp)

		// update column
		o := *c.ColumnName
		p := c.pascal()
		update := fmt.Sprintf(`
	if s.%s != c.%s {
		tmp["%s"] = c.%s
	}`, p, p, o, p)
		columnUpdates = append(columnUpdates, update)
	}

	s.TableStructColumnUpdate = strings.Join(columnUpdates, "")

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
		s.TableStructColumnHey = append(s.TableStructColumnHey, tmp)
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
			s.TableStructColumnHeyValues = append(s.TableStructColumnHeyValues, tmp)
		}

		{
			s96 := string(byte96) // `
			s34 := `"`            // "
			s.TableStructColumnHeyFieldSlice = strings.Join(field, ", ")
			switch s.TypeDriver() {
			case DriverMysql:
				s.TableStructColumnHeyFieldSlice = strings.ReplaceAll(s.TableStructColumnHeyFieldSlice, s34, s96)
			}
			if strings.Index(s.TableStructColumnHeyFieldSlice, s96) >= 0 {
				s.TableStructColumnHeyFieldSliceValue = hey.ConcatString(s34, s.TableStructColumnHeyFieldSlice, s34)
			} else {
				s.TableStructColumnHeyFieldSliceValue = hey.ConcatString(s96, s.TableStructColumnHeyFieldSlice, s96)
			}
		}

		s.TableStructColumnHeyValuesAccess = fmt.Sprintf("[]string{\n\t\t%s\n\t}", strings.Join(fieldAccess, "\n\t\t"))

		fieldAccessMap := fieldAccess[:]
		for k, v := range fieldAccessMap {
			fieldAccessMap[k] = strings.Replace(v, ",", ":{},", 1)
		}
		s.TableStructColumnHeyValuesAccessMap = fmt.Sprintf("map[string]struct{}{\n\t\t%s\n\t}", strings.Join(fieldAccessMap, "\n\t\t"))
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
			s.TableColumnAutoIncr = fmt.Sprintf("[]string{ s.%s }", pascal(autoIncrement[0]))
		} else {
			s.TableColumnAutoIncr = "nil"
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
		s.TableColumnCreatedAt = cs(created...)
		s.TableColumnUpdatedAt = cs(updated...)
		s.TableColumnDeletedAt = cs(deleted...)
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
		s.TableStructColumnReq = append(s.TableStructColumnReq, tmp)
	}

	return
}

func (s *DbModel) Write() (err error) {
	// model.go
	{
		temp := NewTemplate("tmpl_wire", tmplWire)
		pkg := "model"

		buf := bytes.NewBuffer(nil)
		s.buildWire(pkg)
		if err = temp.Execute(buf, s); err != nil {
			return
		}

		filename := pathJoin(s.OutputDirectory, pkg, fmt.Sprintf("%s.go", pkg))
		if err = s.WriteFile(buf, filename); err != nil {
			return
		}
	}

	// model_schema.go
	{
		tmpModelSchema := NewTemplate("tmpl_model_schema", tmplModelSchema)
		tmpModelSchemaContent := NewTemplate("tmpl_model_schema_content", tmplModelSchemaContent)

		modelSchemaFilename := pathJoin(s.OutputDirectory, "model", "model_schema.go")
		modelSchemaBuffer := bytes.NewBuffer(nil)

		modelTableCreateFilename := pathJoin(s.OutputDirectory, "model", "table_create.sql")
		modelTableCreateBuffer := bytes.NewBuffer(nil)

		for _, table := range s.caller.Tables() {
			{
				// for table ddl
				ddl := table.DDL
				for strings.HasSuffix(ddl, "\n") {
					ddl = strings.TrimSuffix(ddl, "\n")
				}
				if _, err = modelTableCreateBuffer.WriteString(ddl); err != nil {
					return
				}
				if !strings.HasSuffix(ddl, ";") {
					if _, err = modelTableCreateBuffer.WriteString(";"); err != nil {
						return
					}
				}
				if _, err = modelTableCreateBuffer.WriteString("\n"); err != nil {
					return
				}
			}
			modelSchemaContentBuffer := bytes.NewBuffer(nil)
			s.clean()
			s.createModelSchemaTable(table)
			if err = tmpModelSchemaContent.Execute(modelSchemaContentBuffer, s); err != nil {
				return
			}
			modelSchemaContentFilename := pathJoin(s.OutputDirectory, "model", fmt.Sprintf("db_%s.go", *table.TableName))
			if err = s.WriteFile(modelSchemaContentBuffer, modelSchemaContentFilename); err != nil {
				return
			}

		}

		if err = tmpModelSchema.Execute(modelSchemaBuffer, s); err != nil {
			return
		}

		if err = s.WriteFile(modelSchemaBuffer, modelSchemaFilename); err != nil {
			return
		}
		if err = s.WriteFile(modelTableCreateBuffer, modelTableCreateFilename); err != nil {
			return
		}

	}

	return
}

type DbData struct {
	*Basis

	// data
	DataImportModelPackageName string // data.go data import model package name
	DataMapListDefine          string // data_schema.go tables define
	DataMapListParams          string // data_schema.go tables params
	DataMapListAssign          string // data_schema.go tables assign
	DataMapListStorage         string // data_schema.go tables storage
	DataMapListSlice           string // data_schema.go tables slice

	DataCustomMethod string // data custom methods
}

func (s *DbData) clean() {
	s.DataMapListDefine = ""
	s.DataMapListParams = ""
	s.DataMapListAssign = ""
	s.DataMapListStorage = ""
	s.DataMapListSlice = ""
}

func NewDbData(basis *Basis) *DbData {
	return &DbData{Basis: basis}
}

func (s *DbData) createDataSchemaTable(table *SysTable) {
	s.TableNamePascal = table.pascal()
	s.TableName = *table.TableName
	if table.TableComment != nil {
		s.TableComment = *table.TableComment
	}
	if table.TableSchema != nil {
		s.TableNameSchema = fmt.Sprintf("%s.%s", *table.TableSchema, s.TableName)
	}
	return
}

type DataSchemaContentCustom struct {
	TableNamePascal      string
	TableFieldNamePascal string
}

func (s *DbData) customMethod(table *SysTable) (buffer *bytes.Buffer, err error) {
	customFields := s.parseCustomTableFields()
	buffer = bytes.NewBuffer(nil)
	fields := make([]string, 0)
	if tmp, ok := customFields[*table.TableName]; ok {
		fields = append(fields, tmp...)
	}

	// the primary key of the table, 这里会依赖model
	if table.TableAutoIncrement != "" {
		had := false
		for _, v := range fields {
			if v == table.TableAutoIncrement {
				had = true
				break
			}
		}
		if !had {
			val := make([]string, 1, 1+len(fields))
			val[0] = table.TableAutoIncrement
			val = append(val, fields...)
			fields = val
		}
	}

	for _, field := range fields {
		value := &DataSchemaContentCustom{
			TableNamePascal:      s.TableNamePascal,
			TableFieldNamePascal: pascal(field),
		}
		temp := NewTemplate("data_schema_content_custom", tmplDataSchemaContentCustom)
		if err = temp.Execute(buffer, value); err != nil {
			return
		}
	}
	return
}

func (s *DbData) Write() (err error) {
	// data.go
	{
		temp := NewTemplate("tmpl_wire", tmplWire)
		pkg := "data"
		filename := pathJoin(s.OutputDirectory, pkg, fmt.Sprintf("%s.go", pkg))
		buffer := bytes.NewBuffer(nil)
		s.buildWire(pkg)
		if err = temp.Execute(buffer, s); err != nil {
			return
		}
		if err = s.WriteFile(buffer, filename); err != nil {
			return
		}
	}

	// data_schema.go
	{
		tmpDataSchema := NewTemplate("data_schema", tmplDataSchema)
		tmpDataSchemaContent := NewTemplate("data_schema_content", tmplDataSchemaContent)

		tables := s.caller.Tables()
		s.DataImportModelPackageName = s.ImportModelPackageName
		var customMethodBuffer *bytes.Buffer
		for _, table := range tables {
			s.clean()
			s.createDataSchemaTable(table)
			schemaContentBuffer := bytes.NewBuffer(nil)
			if customMethodBuffer, err = s.customMethod(table); err != nil {
				return
			}
			s.DataCustomMethod = customMethodBuffer.String()
			if err = tmpDataSchemaContent.Execute(schemaContentBuffer, s); err != nil {
				return err
			}
			schemaContentFilename := pathJoin(s.OutputDirectory, "data", fmt.Sprintf("db_%s.go", *table.TableName))
			if err = s.WriteFile(schemaContentBuffer, schemaContentFilename); err != nil {
				return
			}
		}

		filename := pathJoin(s.OutputDirectory, "data", "data_schema.go")
		buffer := bytes.NewBuffer(nil)

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
		s.DataMapListDefine = strings.Join(defines, "\n\t")
		s.DataMapListParams = strings.Join(params, "\n\t")
		s.DataMapListAssign = strings.Join(assigns, "\n\t\t")
		s.DataMapListStorage = strings.Join(storage, "\n\t\t")
		s.DataMapListSlice = strings.Join(slice, "\n\t\t")
		if err = tmpDataSchema.Execute(buffer, s); err != nil {
			return
		}
		if err = s.WriteFile(buffer, filename); err != nil {
			return
		}
	}
	return
}

type DbBiz struct {
	*Basis

	// biz
	BizImportDataPackageName  string // biz.go data import model package name
	BizAllTablesSchemaContent string // biz.go data all tables schema content
}

func NewDbBiz(basis *Basis) *DbBiz {
	return &DbBiz{Basis: basis}
}

func (s *DbBiz) createBizSchemaTable(table *SysTable) {
	s.TableNamePascal = table.pascal()
	s.TableName = *table.TableName
	if table.TableComment != nil {
		s.TableComment = *table.TableComment
	}
	if table.TableSchema != nil {
		s.TableNameSchema = fmt.Sprintf("%s.%s", *table.TableSchema, s.TableName)
	}
	s.TableNameSmallPascal = strings.ToLower(s.TableNamePascal[0:1]) + s.TableNamePascal[1:]
	return
}

func (s *DbBiz) Write() (err error) {
	// biz.tmpl
	{
		temp := NewTemplate("tmpl_wire", tmplWire)
		pkg := "biz"
		filename := pathJoin(s.OutputDirectory, pkg, fmt.Sprintf("%s.tmpl", pkg))
		buffer := bytes.NewBuffer(nil)
		s.buildWire(pkg)
		if err = temp.Execute(buffer, s); err != nil {
			return
		}
		if err = s.WriteFile(buffer, filename); err != nil {
			return
		}
	}

	// biz_schema.tmpl
	{
		tmpBizSchema := NewTemplate("biz_schema", tmplBizSchema)
		tmpBizSchemaContent := NewTemplate("biz_schema_content", tmplBizSchemaContent)

		buffer := bytes.NewBuffer(nil)

		for _, table := range s.caller.Tables() {
			s.createBizSchemaTable(table)
			if err = tmpBizSchemaContent.Execute(buffer, s); err != nil {
				return err
			}
		}

		filename := pathJoin(s.OutputDirectory, "biz", "biz_schema.tmpl")
		bizSchemaBuffer := bytes.NewBuffer(nil)
		s.BizImportDataPackageName = strings.Replace(s.ImportModelPackageName, "model", "data", 1)
		s.BizAllTablesSchemaContent = buffer.String()
		if err = tmpBizSchema.Execute(bizSchemaBuffer, s); err != nil {
			return err
		}
		if err = s.WriteFile(bizSchemaBuffer, filename); err != nil {
			return
		}
	}

	return
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
	DDL                string       `db:"-"`             // 表定义语句
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

func bufferTable(fn func(i int, table *SysTable) string, tables ...*SysTable) *bytes.Buffer {
	buffer := bytes.NewBuffer(nil)
	for index, table := range tables {
		buffer.WriteString(fn(index, table))
	}
	return buffer
}
