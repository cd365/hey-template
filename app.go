package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
	"unsafe"

	"github.com/cd365/hey/v2"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

const (
	Version = "v0.6.0"
)

var (
	BuildTime  = ""
	CommitHash = ""
)

type Helper interface {
	QueryAll() error
	AllTable() []*SysTable
	TableDdl(table *SysTable) error
}

type App struct {
	Version string

	cfg *Config

	way *hey.Way // 数据库连接对象

	helper Helper // 数据接口
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
	return TypeDriver(strings.ToLower(s.cfg.Driver))
}

func (s *App) initial() error {
	s.cfg.Driver = strings.TrimSpace(s.cfg.Driver)
	way, err := hey.NewWay(s.cfg.Driver, s.cfg.DataSourceName)
	if err != nil {
		return err
	}
	s.way = way
	db := way.DB()
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(2)
	db.SetConnMaxIdleTime(time.Minute * 3)
	db.SetConnMaxLifetime(time.Minute * 3)
	switch s.TypeDriver() {
	case DriverMysql:
		s.cfg.DatabaseIdentify = "`"
		s.helper = Mysql(s)
		if s.cfg.TableSchemaName == "" {
			start := strings.Index(s.cfg.DataSourceName, "/")
			if start > -1 {
				end := strings.Index(s.cfg.DataSourceName, "?")
				if end > -1 {
					s.cfg.TableSchemaName = s.cfg.DataSourceName[start+1 : end]
				} else {
					s.cfg.TableSchemaName = s.cfg.DataSourceName[start+1:]
				}
			}
			s.cfg.TableSchemaName = strings.TrimSpace(s.cfg.TableSchemaName)
		}
	case DriverPostgres:
		s.cfg.DatabaseIdentify = `"`
		s.helper = Pgsql(s)
		if s.cfg.TableSchemaName == "" {
			s.cfg.TableSchemaName = "public"
		}
	default:
		return fmt.Errorf("unsupported driver name: %s", s.cfg.Driver)
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

func (s *App) AllTable(all bool) []*SysTable {
	if all {
		return s.helper.AllTable()
	}
	allTable := s.helper.AllTable()
	length := len(allTable)
	result := make([]*SysTable, 0, length)
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
	if TypeDriver(s.cfg.Driver) == DriverPostgres {
		if _, err := s.way.DB().Exec(pgsqlFuncCreate); err != nil {
			return err
		}
		defer func() { _, _ = s.way.DB().Exec(pgsqlFuncDrop) }()
	}
	if err := s.helper.QueryAll(); err != nil {
		return err
	}
	for _, table := range s.AllTable(true) {
		if err := s.helper.TableDdl(table); err != nil {
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

func upper(str string) string {
	return strings.ToUpper(str)
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
	if s.app.cfg.UsingTableSchemaName && s.app.cfg.TableSchemaName != "" {
		tmp.OriginNameWithPrefix = fmt.Sprintf("%s.%s", s.app.cfg.TableSchemaName, *s.TableName)
	}
	if s.TableComment != nil && *s.TableComment != "" {
		tmp.Comment = *s.TableComment
	}
	return tmp
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

func (s *SysColumn) upper() string {
	if s.ColumnName == nil {
		return ""
	}
	return upper(*s.ColumnName)
}

func (s *SysColumn) comment() string {
	if s.ColumnComment == nil {
		return ""
	}
	return *s.ColumnComment
}
