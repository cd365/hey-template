package main

import (
	"bytes"
	"fmt"
	"github.com/cd365/hey/v2"
	"strings"
)

type TmplWire struct {
	Version string // 模板版本
	Package string // 包名
	Content string // 内容
}

func (s *App) MakeTmplWire(pkg string, suffixName string, customLines ...string) error {
	w := &TmplWire{
		Version: s.Version,
		Package: pkg,
	}
	temp := NewTemplate("tmp_wire", tmplWire)
	text := bytes.NewBuffer(nil)
	newTable := func(i int, table *SysTable) string {
		tmp := fmt.Sprintf("New%s%s,", table.pascal(), suffixName)
		if i > 0 {
			tmp = fmt.Sprintf("\n\t%s", tmp)
		}
		comment := table.comment()
		if comment != "" {
			tmp = fmt.Sprintf("%s // %s", tmp, comment)
		}
		return tmp
	}
	buffer := bytes.NewBuffer(nil)
	for index, table := range s.AllTable(false) {
		buffer.WriteString(newTable(index, table))
	}
	for _, v := range customLines {
		buffer.WriteString(v)
	}
	w.Content = buffer.String()
	if err := temp.Execute(text, w); err != nil {
		return err
	}
	suffix := ".go"
	switch pkg {
	case "biz":
		suffix = ".tmp"
	}
	filename := pathJoin(s.cfg.TemplateOutputDirectory, w.Package, fmt.Sprintf("%s%s", w.Package, suffix))
	if err := s.WriteFile(text, filename); err != nil {
		return err
	}
	return nil
}

type TableColumnPrimaryKey struct {
	OriginNamePascal      string
	PrimaryKeyPascal      string
	PrimaryKeySmallPascal string
	PrimaryKeyUpper       string
}

type TmplTableModel struct {
	table *SysTable

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

func (s *TmplTableModel) Make() {

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
			switch s.table.app.TypeDriver() {
			case DriverMysql:
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
		cm := make(map[string]*SysColumn)
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
			s.ColumnAutoIncr = fmt.Sprintf("[]string{ s.%s }", upper(autoIncrement[0]))
		} else {
			s.ColumnAutoIncr = "nil"
		}
		cs := func(cols ...string) string {
			length := len(cols)
			if length == 0 {
				return "nil"
			}
			for i := 0; i < length; i++ {
				cols[i] = fmt.Sprintf("s.%s", upper(cols[i]))
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
			s.StructColumnAddPrimaryKey = fmt.Sprintf("func (s %sInsert) PrimaryKey() interface{} {\n\treturn nil\n}", s.table.pascal())
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
			s.StructColumnPrimaryKey = fmt.Sprintf("type %sPrimaryKey struct {\n\t%s *%s `json:\"%s\" db:\"-\" validate:\"omitempty,min=1\"`%s\n}\n\nfunc (s %sPrimaryKey) PrimaryKey() interface{} {\n\t if s.%s != nil {\n\treturn *s.%s\n\t}\n\treturn nil\n}",
				tablePascal,
				columnPascal,
				c.databaseTypeToGoType(),
				underline(*c.ColumnName),
				comment,
				tablePascal,
				columnPascal,
				columnPascal,
			)
			// append Primary-Key define
			s.StructColumnMod = append(s.StructColumnMod, fmt.Sprintf("\t%sPrimaryKey\n", tablePascal))
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
			PrimaryKeyPascal:      pascal(s.table.TableFieldSerial),
			PrimaryKeySmallPascal: pascalSmall(s.table.TableFieldSerial),
			PrimaryKeyUpper:       strings.ToUpper(s.table.TableFieldSerial),
		}
		if err := tmpl.Execute(buffer, data); err != nil {
			return
		} else {
			s.PrimaryKey = buffer.String()
		}
	}

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
	if s.cfg.UsingWire {
		// model.go
		if err := s.MakeTmplWire("model", "Schema", "\n\tNewSchemaAll, // schema all"); err != nil {
			return err
		}
	}

	tables := s.AllTable(false)

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
			if _, err := modelTableCreateBuffer.WriteString(fmt.Sprintf("/* %s (%s) */\n", *table.TableName, *table.TableComment)); err != nil {
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
		tmp := table.TmplTableModel()
		tmp.Make()
		if err := tmpModelSchemaContent.Execute(modelSchemaContentBuffer, tmp); err != nil {
			return err
		}
		modelSchemaContentFilename := pathJoin(s.cfg.TemplateOutputDirectory, "model", fmt.Sprintf("%s%s%s%s", tableFilenamePrefix, *table.TableName, tableFilenameSuffix, tableFilenameGo))
		if err := s.WriteFile(modelSchemaContentBuffer, modelSchemaContentFilename); err != nil {
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
		schemaName := "TableSchema"
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
		if err := s.WriteFile(modelSchemaBuffer, modelSchemaFilename); err != nil {
			return err
		}
	}

	// model_schema_custom.go
	{
		if err := tmpModelSchemaCustom.Execute(modelSchemaCustomBuffer, s); err != nil {
			return err
		}

		// if _, err := os.Stat(modelSchemaCustomFilename); err != nil {
		if err := s.WriteFile(modelSchemaCustomBuffer, modelSchemaCustomFilename); err != nil {
			return err
		}
		// }
	}

	// table_create.sql
	{
		if err := s.WriteFile(modelTableCreateBuffer, modelTableCreateFilename); err != nil {
			return err
		}
	}

	return nil
}
