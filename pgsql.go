package main

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

type pgsql1 struct {
	app    *App
	tables []*SysTable
}

func Pgsql(app *App) Ber {
	return &pgsql1{app: app}
}

func (s *pgsql1) QueryAll() (err error) {
	schema := s.app.config.TableSchemaName
	prepare := "SELECT table_schema, table_name FROM information_schema.tables WHERE ( table_schema = ? AND table_type = 'BASE TABLE' ) ORDER BY table_name ASC"
	if err = s.app.way.TakeAll(&s.tables, prepare, schema); err != nil {
		return
	}
	once := &sync.Once{}
	wg := &sync.WaitGroup{}
	for _, table := range s.tables {
		table.app = s.app
		wg.Add(1)
		go func(table *SysTable) {
			defer wg.Done()
			columns, qer := s.queryColumns(schema, table)
			if qer != nil {
				once.Do(func() { err = qer })
				return
			}
			table.Column = columns
			if qer = s.queryComment(schema, table); qer != nil {
				once.Do(func() { err = qer })
			}
		}(table)
	}
	wg.Wait()
	return
}

func (s *pgsql1) queryComment(schema string, table *SysTable) (err error) {
	if table.TableName == nil || schema == "" {
		return
	}
	prepare := "SELECT cast(obj_description(relfilenode, 'pg_class') AS VARCHAR) AS table_comment FROM pg_tables LEFT OUTER JOIN pg_class ON pg_tables.tablename = pg_class.relname WHERE ( pg_tables.schemaname = ? AND pg_tables.tablename = ? ) ORDER BY pg_tables.schemaname ASC LIMIT 1;"
	if err = s.app.way.Query(func(rows *sql.Rows) (err error) {
		if !rows.Next() {
			return
		}
		comment := sql.NullString{}
		if err = rows.Scan(&comment); err != nil {
			return
		}
		if comment.Valid {
			table.TableComment = &comment.String
		} else {
			table.TableComment = new(string)
		}
		return
	}, prepare, schema, *table.TableName); err != nil {
		return
	}
	if table.TableComment == nil {
		table.TableComment = new(string)
	}
	return
}

func (s *pgsql1) queryColumns(schema string, table *SysTable) (list []*SysColumn, err error) {
	if schema == "" || table == nil || table.TableName == nil || *table.TableName == "" {
		return
	}
	prepare := "SELECT table_schema, table_name, column_name, ordinal_position, column_default, is_nullable, data_type, character_maximum_length, character_octet_length, numeric_precision, numeric_scale, character_set_name, collation_name FROM information_schema.columns WHERE ( table_schema = ? AND table_name = ? ) ORDER BY ordinal_position ASC"
	err = s.app.way.Query(func(rows *sql.Rows) (err error) {
		for rows.Next() {
			tmp := &SysColumn{}
			if err = rows.Scan(
				&tmp.TableSchema,
				&tmp.TableName,
				&tmp.ColumnName,
				&tmp.OrdinalPosition,
				&tmp.ColumnDefault,
				&tmp.IsNullable,
				&tmp.DataType,
				&tmp.CharacterMaximumLength,
				&tmp.CharacterOctetLength,
				&tmp.NumericPrecision,
				&tmp.NumericScale,
				&tmp.CharacterSetName,
				&tmp.ColumnComment,
			); err != nil {
				return
			}
			list = append(list, tmp)
		}
		return
	}, prepare, schema, *table.TableName)
	if err != nil {
		return
	}
	for k, v := range list {
		v.table = table
		if v.ColumnName == nil || *v.ColumnName == "" {
			continue
		}
		// query column comment
		// SELECT a.attnum AS id, a.attname AS column_name, t.typname AS type_basic, SUBSTRING(FORMAT_TYPE(a.atttypid, a.atttypmod) FROM '(.*)') AS type_sql, a.attnotnull AS not_null, d.description AS comment FROM pg_class c, pg_attribute a, pg_type t, pg_description d WHERE ( c.relname = 'TABLE_NAME' AND a.attnum > 0 AND a.attrelid = c.oid AND a.atttypid = t.oid AND d.objoid = a.attrelid AND d.objsubid = a.attnum ) ORDER BY id ASC;
		err = s.app.way.Query(func(rows *sql.Rows) (err error) {
			if !rows.Next() {
				return
			}
			tmp := ""
			if err = rows.Scan(&tmp); err != nil {
				return
			}
			list[k].ColumnComment = &tmp
			return
		}, "SELECT d.description AS column_comment FROM pg_class c, pg_attribute a, pg_type t, pg_description d WHERE ( c.relname = ? AND a.attname = ? AND a.attnum > 0 AND a.attrelid = c.oid AND a.atttypid = t.oid AND d.objoid = a.attrelid AND d.objsubid = a.attnum ) ORDER BY a.attnum ASC LIMIT 1;", *table.TableName, *v.ColumnName)
		if err != nil {
			return
		}
		if v.ColumnComment == nil {
			v.ColumnComment = new(string)
		}
	}
	return
}

func (s *pgsql1) AllTable() []*SysTable {
	return s.tables
}

var pgSeq = regexp.MustCompile(`^nextval\('([A-Za-z0-9_]+)'::regclass\)$`)

func (s *pgsql1) TableDdl(table *SysTable) error {
	var createSequence string
	for _, c := range table.Column {
		if c.ColumnDefault == nil {
			continue
		}
		if strings.Contains(*c.ColumnDefault, "\"") {
			*c.ColumnDefault = strings.ReplaceAll(*c.ColumnDefault, "\"", "")
		}
		if pgSeq.MatchString(*c.ColumnDefault) {
			result := pgSeq.FindAllStringSubmatch(*c.ColumnDefault, -1)
			if len(result) == 1 && len(result[0]) == 2 && result[0][1] != "" {
				createSequence = fmt.Sprintf("CREATE SEQUENCE IF NOT EXISTS %s START 1;\n", result[0][1])
				table.TableFieldSerial = *c.ColumnName
			}
		}
	}
	prepare := fmt.Sprintf("SELECT show_create_table_schema('%s', '%s')", *table.TableSchema, *table.TableName)
	result := ""
	err := s.app.way.Query(func(rows *sql.Rows) error {
		for rows.Next() {
			if err := rows.Scan(&result); err != nil {
				return err
			}
		}
		return nil
	}, prepare)
	if err != nil {
		return err
	}
	result = strings.ReplaceAll(result, "CREATE TABLE", "CREATE TABLE IF NOT EXISTS")
	result = strings.ReplaceAll(result, "CREATE INDEX", "CREATE INDEX IF NOT EXISTS")
	result = strings.ReplaceAll(result, "CREATE UNIQUE INDEX", "CREATE UNIQUE INDEX IF NOT EXISTS")
	result = createSequence + result
	table.DDL = result
	return nil
}
