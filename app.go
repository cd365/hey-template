package main

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
	"unsafe"

	"github.com/cd365/hey/pgsql"

	"github.com/cd365/hey"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

const (
	Version = "v0.5.0"
)

var (
	BuildTime  = ""
	CommitHash = ""
)

type Ber interface {
	QueryAll() error
	AllTable() []*SysTable
	TableDdl(table *SysTable) error
}

type App struct {
	Version string

	Driver            string // 数据库驱动名称 mysql|postgres
	DataSourceName    string // 数据源名称 mysql=>root:112233@tcp(127.0.0.1:3306)/hello?charset=utf8mb4&collation=utf8mb4_unicode_ci&timeout=90s pgsql=>postgres://postgres:112233@[::1]:5432/hello?sslmode=disable
	TablePrefixName   string // 数据库前缀名称 public
	PrefixPackageName string // 包导入前缀 main
	TablePrefix       bool   // 表名是否使用前缀
	FieldSerial       string // 表自动递增字段(只有一个字段自动递增)
	FieldPrimaryKey   string // 表主键字段列表, 多个","隔开
	FieldCreatedAt    string // 创建时间戳字段列表, 多个","隔开
	FieldUpdatedAt    string // 更新时间戳字段列表, 多个","隔开
	FieldDeletedAt    string // 删除时间戳字段列表, 多个","隔开
	OutputDirectory   string // 输出路径
	Admin             bool   // 管理端快速增删改代码
	AdminUrlPrefix    string // 管理端路由前缀
	Index             bool   // C端快速增删改代码
	IndexUrlPrefix    string // C端路由前缀

	Identify string // 数据库标识符号 mysql: ` postgres: "

	way *hey.Way // 数据库连接对象
	ber Ber      // 数据接口
}

var cmd = &App{
	Version: Version,
}

type TypeDriver string

const (
	DriverMysql    TypeDriver = "mysql"
	DriverPostgres TypeDriver = "postgres"
)

func (s *App) TypeDriver() TypeDriver {
	return TypeDriver(strings.ToLower(s.Driver))
}

func (s *App) initial() error {
	s.Driver = strings.TrimSpace(s.Driver)
	conn, err := sql.Open(s.Driver, s.DataSourceName)
	if err != nil {
		return err
	}
	conn.SetMaxOpenConns(8)
	conn.SetMaxIdleConns(2)
	conn.SetConnMaxIdleTime(time.Minute * 3)
	conn.SetConnMaxLifetime(time.Minute * 3)
	switch s.TypeDriver() {
	case DriverMysql:
		s.Identify = "`"
		s.way = hey.NewWay(conn)
		s.ber = Mysql(s)
		if s.TablePrefixName == "" {
			start := strings.Index(s.DataSourceName, "/")
			if start > -1 {
				end := strings.Index(s.DataSourceName, "?")
				if end > -1 {
					s.TablePrefixName = s.DataSourceName[start+1 : end]
				} else {
					s.TablePrefixName = s.DataSourceName[start+1:]
				}
			}
			s.TablePrefixName = strings.TrimSpace(s.TablePrefixName)
		}
	case DriverPostgres:
		s.Identify = `"`
		s.way = hey.NewWay(conn, hey.WithPrepare(pgsql.Prepare))
		s.ber = Pgsql(s)
		if s.TablePrefixName == "" {
			s.TablePrefixName = "public"
		}
	default:
		return fmt.Errorf("unsupported driver name: %s", s.Driver)
	}
	return nil
}

func (s *App) WriteFile(reader io.Reader, filename string) error {
	fil, err := createFile(filename)
	if err != nil {
		return err
	}
	defer func() { _ = fil.Close() }()
	_, err = io.Copy(fil, reader)
	return err
}

func (s *App) BuildAll() error {
	if err := s.initial(); err != nil {
		return err
	}
	if TypeDriver(s.Driver) == DriverPostgres {
		if _, err := s.way.DB().Exec(pgsqlFuncCreate); err != nil {
			return err
		}
		defer func() { _, _ = s.way.DB().Exec(pgsqlFuncDrop) }()
	}
	if err := s.ber.QueryAll(); err != nil {
		return err
	}
	for _, table := range s.ber.AllTable() {
		if err := s.ber.TableDdl(table); err != nil {
			return err
		}
	}
	writer := make([]func() error, 0, 8)
	writer = append(writer, s.Model)
	writer = append(writer, s.Data)
	writer = append(writer, s.Biz)
	if s.Admin {
		writer = append(writer, s.Asc)
	}
	if s.Index {
		writer = append(writer, s.Can)
	}
	for _, w := range writer {
		if err := w(); err != nil {
			return err
		}
	}
	return nil
}

const (
	byteUnderline        byte = '_'
	byte34                    = '"'
	byte96                    = '`'
	templateLeft              = "{{{"
	templateRight             = "}}}"
	tableFilenamePrefix       = "zzz_"
	tableFilenameSuffix       = "_aaa"
	tableFilenameSuffix1      = "_aab"
	tableFilenameSuffix2      = "_aac"
	tableFilenameGo           = ".go"
	tableFilenameTmp          = ".tmp"
)

// tableFilenameGoTmp xxx.go => xxx.tmp
func tableFilenameGoTmp(pathDirectory string) string {
	return fmt.Sprintf("%s%s", strings.TrimSuffix(pathDirectory, tableFilenameGo), tableFilenameTmp)
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

func underline(str string) string {
	length := len(str)
	tmp := make([]byte, 0, length)
	for i := 0; i < length; i++ {
		if str[i] >= 'A' && str[i] <= 'Z' {
			if i > 0 {
				tmp = append(tmp, byteUnderline)
			}
			tmp = append(tmp, str[i]+32)
		} else {
			tmp = append(tmp, str[i])
		}
	}
	return *(*string)(unsafe.Pointer(&tmp))
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
	app              *App         `db:"-"`
	TableSchema      *string      `db:"table_schema"`  // 数据库名
	TableName        *string      `db:"table_name"`    // 表名
	TableComment     *string      `db:"table_comment"` // 表注释
	TableFieldSerial string       `db:"-"`             // 表自动递增字段
	Column           []*SysColumn `db:"-"`             // 表中的所有字段
	DDL              string       `db:"-"`             // 表定义语句
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

func (s *SysTable) TmplTableModel() *TmplTableModel {
	tmp := &TmplTableModel{
		table:                s,
		Version:              s.app.Version,
		OriginName:           *s.TableName,
		OriginNamePascal:     pascal(*s.TableName),
		OriginNameWithPrefix: *s.TableName,
		OriginNameCamel:      s.pascalSmall(),
		Comment:              *s.TableName,
	}
	if s.app.TablePrefix && s.app.TablePrefixName != "" {
		tmp.OriginNameWithPrefix = fmt.Sprintf("%s.%s", s.app.TablePrefixName, *s.TableName)
	}
	if s.TableComment != nil && *s.TableComment != "" {
		tmp.Comment = *s.TableComment
	}
	return tmp
}

func (s *SysTable) TmplTableData() *TmplTableData {
	model := s.TmplTableModel()
	return &TmplTableData{
		table:                s,
		Version:              model.Version,
		OriginName:           model.OriginName,
		OriginNamePascal:     model.OriginNamePascal,
		OriginNameWithPrefix: model.OriginNameWithPrefix,
		OriginNameCamel:      model.OriginNameCamel,
		Comment:              model.Comment,
		PrefixPackage:        s.app.PrefixPackageName,
	}
}

func (s *SysTable) TmplTableBiz() *TmplTableBiz {
	model := s.TmplTableModel()
	return &TmplTableBiz{
		table:                s,
		Version:              model.Version,
		OriginName:           model.OriginName,
		OriginNamePascal:     model.OriginNamePascal,
		OriginNameWithPrefix: model.OriginNameWithPrefix,
		OriginNameCamel:      model.OriginNameCamel,
		Comment:              model.Comment,
		PrefixPackage:        s.app.PrefixPackageName,
	}
}

func (s *SysTable) TmplTableAsc() *TmplTableAsc {
	model := s.TmplTableModel()
	return &TmplTableAsc{
		table:                s,
		Version:              model.Version,
		OriginName:           model.OriginName,
		OriginNamePascal:     model.OriginNamePascal,
		OriginNameWithPrefix: model.OriginNameWithPrefix,
		OriginNameCamel:      model.OriginNameCamel,
		Comment:              model.Comment,
		PrefixPackage:        s.app.PrefixPackageName,
		FileNamePrefix:       tableFilenamePrefix,
	}
}

func (s *SysTable) TmplTableCan() *TmplTableCan {
	model := s.TmplTableModel()
	return &TmplTableCan{
		table:                s,
		Version:              model.Version,
		OriginName:           model.OriginName,
		OriginNamePascal:     model.OriginNamePascal,
		OriginNameWithPrefix: model.OriginNameWithPrefix,
		OriginNameCamel:      model.OriginNameCamel,
		Comment:              model.Comment,
		PrefixPackage:        s.app.PrefixPackageName,
		FileNamePrefix:       tableFilenamePrefix,
	}
}

// SysColumn 表字段结构
type SysColumn struct {
	table                  *SysTable `db:"-"`
	TableSchema            *string   `db:"table_schema"`             // 数据库名
	TableName              *string   `db:"table_name"`               // 表名
	ColumnName             *string   `db:"column_name"`              // 列名
	OrdinalPosition        *int      `db:"ordinal_position"`         // 列序号
	ColumnDefault          *string   `db:"column_default"`           // 列默认值
	IsNullable             *string   `db:"is_nullable"`              // 是否允许列值为null
	DataType               *string   `db:"data_type"`                // 列数据类型
	CharacterMaximumLength *int      `db:"character_maximum_length"` // 字符串最大长度
	CharacterOctetLength   *int      `db:"character_octet_length"`   // 文本字符串字节最大长度
	NumericPrecision       *int      `db:"numeric_precision"`        // 整数最长长度|小数(整数+小数)合计长度
	NumericScale           *int      `db:"numeric_scale"`            // 小数精度长度
	CharacterSetName       *string   `db:"character_set_name"`       // 字符集名称
	CollationName          *string   `db:"collation_name"`           // 校对集名称
	ColumnComment          *string   `db:"column_comment"`           // 列注释
	ColumnType             *string   `db:"column_type"`              // 列类型
	ColumnKey              *string   `db:"column_key"`               // 列索引 '', 'PRI', 'UNI', 'MUL'
	Extra                  *string   `db:"extra"`                    // 列额外属性 auto_increment
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
	if nullable {
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

func (s *SysColumn) pascalSmall() string {
	name := s.pascal()
	if len(name) == 0 {
		return ""
	}
	return strings.ToLower(name[0:1]) + name[1:]
}

func (s *SysColumn) comment() string {
	if s.ColumnComment == nil {
		return ""
	}
	return *s.ColumnComment
}
