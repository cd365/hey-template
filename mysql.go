package main

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

var (
	autoIncrementRegexpReplace = regexp.MustCompile(`(AUTO_INCREMENT|auto_increment)=\d+`)
)

type mysql struct {
	app    *App
	tables []*SysTable
}

func Mysql(app *App) Ber {
	return &mysql{app: app}
}

func (s *mysql) QueryAll() (err error) {
	schema := s.app.TablePrefixName
	prepare := "SELECT TABLE_SCHEMA AS table_schema, TABLE_NAME AS table_name, TABLE_COMMENT AS table_comment FROM information_schema.TABLES WHERE TABLE_TYPE = 'BASE TABLE' AND TABLE_SCHEMA = ? ORDER BY TABLE_NAME ASC;"
	if err = s.app.way.ScanAll(&s.tables, prepare, schema); err != nil {
		return
	}
	once := &sync.Once{}
	wg := &sync.WaitGroup{}
	for _, table := range s.tables {
		table.app = s.app
		wg.Add(1)
		go func(table *SysTable) {
			defer wg.Done()
			columns, qer := s.getAllColumns(schema, table)
			if qer != nil {
				once.Do(func() { err = qer })
				return
			}
			table.Column = columns
		}(table)
	}
	wg.Wait()
	return
}

func (s *mysql) getAllColumns(schema string, table *SysTable) (list []*SysColumn, err error) {
	if schema == "" || table == nil || table.TableName == nil || *table.TableName == "" {
		return
	}
	prepare := "SELECT TABLE_SCHEMA AS table_schema, TABLE_NAME AS table_name, COLUMN_NAME AS column_name, ORDINAL_POSITION AS ordinal_position, COLUMN_DEFAULT AS column_default, IS_NULLABLE AS is_nullable, DATA_TYPE AS data_type, CHARACTER_MAXIMUM_LENGTH AS character_maximum_length, CHARACTER_OCTET_LENGTH AS character_octet_length, NUMERIC_PRECISION AS numeric_precision, NUMERIC_SCALE AS numeric_scale, CHARACTER_SET_NAME AS character_set_name, COLLATION_NAME AS collation_name, COLUMN_COMMENT AS column_comment, COLUMN_TYPE AS column_type, COLUMN_KEY AS column_key, EXTRA AS extra FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY ordinal_position ASC"
	list = make([]*SysColumn, 0)
	if err = s.app.way.ScanAll(&list, prepare, schema, *table.TableName); err != nil {
		return
	}
	for k, v := range list {
		v.table = table
		if v.ColumnComment == nil {
			list[k].ColumnComment = new(string)
		}
	}
	return
}

func (s *mysql) AllTable() []*SysTable {
	return s.tables
}

func (s *mysql) TableDdl(table *SysTable) error {
	for _, c := range table.Column {
		if c.Extra != nil && strings.ToLower(*c.Extra) == "auto_increment" {
			table.TableFieldSerial = *c.ColumnName
		}
	}
	prepare := fmt.Sprintf("SHOW CREATE TABLE %s.%s", *table.TableSchema, *table.TableName)
	name, result := "", ""
	err := s.app.way.Query(func(rows *sql.Rows) error {
		for rows.Next() {
			if err := rows.Scan(&name, &result); err != nil {
				return err
			}
		}
		return nil
	}, prepare)
	if err != nil {
		return err
	}
	table.DDL = strings.ReplaceAll(result, "CREATE TABLE", "CREATE TABLE IF NOT EXISTS")
	table.DDL = autoIncrementRegexpReplace.ReplaceAllString(table.DDL, "${1}=1")
	return nil
}
