package dbs

import (
	"sync"

	"github.com/cd365/hey"
)

type Mysql struct {
	param  *Param
	way    *hey.Way
	tables []*SysTable
}

func NewMysql(
	param *Param,
	way *hey.Way,
) *Mysql {
	return &Mysql{
		param: param,
		way:   way,
	}
}

func (s *Mysql) Queries() (err error) {
	schema := s.param.DatabaseSchemaName
	prepare := "SELECT TABLE_SCHEMA AS table_schema, TABLE_NAME AS table_name, TABLE_COMMENT AS table_comment FROM information_schema.TABLES WHERE TABLE_TYPE = 'BASE TABLE' AND TABLE_SCHEMA = ? ORDER BY TABLE_NAME ASC;"
	if err = s.way.ScanAll(&s.tables, prepare, schema); err != nil {
		return
	}
	once := &sync.Once{}
	wg := &sync.WaitGroup{}
	for _, table := range s.tables {
		table.way = s.way
		wg.Add(1)
		go func(table *SysTable) {
			defer wg.Done()
			columns, e := s.getAllColumns(schema, *table.TableName)
			if e != nil {
				once.Do(func() { err = e })
			} else {
				table.Column = columns
			}
		}(table)
	}
	wg.Wait()
	return
}

func (s *Mysql) getAllColumns(schema string, table string) (list []*SysColumn, err error) {
	if s.way == nil || schema == "" || table == "" {
		return
	}
	prepare := "SELECT TABLE_SCHEMA AS table_schema, TABLE_NAME AS table_name, COLUMN_NAME AS column_name, ORDINAL_POSITION AS ordinal_position, COLUMN_DEFAULT AS column_default, IS_NULLABLE AS is_nullable, DATA_TYPE AS data_type, CHARACTER_MAXIMUM_LENGTH AS character_maximum_length, CHARACTER_OCTET_LENGTH AS character_octet_length, NUMERIC_PRECISION AS numeric_precision, NUMERIC_SCALE AS numeric_scale, CHARACTER_SET_NAME AS character_set_name, COLLATION_NAME AS collation_name, COLUMN_COMMENT AS column_comment, COLUMN_TYPE AS column_type, COLUMN_KEY AS column_key, EXTRA AS extra FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY ordinal_position ASC"
	list = make([]*SysColumn, 0)
	if err = s.way.ScanAll(&list, prepare, schema, table); err != nil {
		return
	}
	for k, v := range list {
		if v.ColumnComment == nil {
			list[k].ColumnComment = new(string)
		}
	}
	return
}

func (s *Mysql) Tables() []*SysTable {
	return s.tables
}
