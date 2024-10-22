package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/cd365/hey"
)

type TmplWire struct {
	Version string // 模板版本
	Package string // 包名
	Content string // 内容
}

func (s *App) MakeTmplWire(pkg string, customLines ...string) error {
	w := &TmplWire{
		Version: s.Version,
		Package: pkg,
	}
	temp := NewTemplate("tmp_wire", tmplWire)
	text := bytes.NewBuffer(nil)
	newTable := func(i int, table *SysTable) string {
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
	buffer := bytes.NewBuffer(nil)
	for index, table := range s.ber.AllTable() {
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
	filename := pathJoin(s.OutputDirectory, w.Package, fmt.Sprintf("%s%s", w.Package, suffix))
	if err := s.WriteFile(text, filename); err != nil {
		return err
	}
	return nil
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
	StructColumn                   []string // 表结构体字段定义 ==> Name string `json:"name" db:"name"` // 名称
	StructColumnHey                []string // 表结构体字段关系定义 ==> Name string // name 名称
	StructColumnHeyFieldSlice      string   // NewHey.Field ==> // []string{"id", "name"}
	StructColumnHeyFieldSliceValue string   // NewHey.FieldStr ==> // `"id", "name"` || "`id`, `name`"
	StructColumnMod                []string // 表结构体字段定义 ==> Name *string `json:"name" db:"name"` // 名称
	StructColumnAdd                []string // 表结构体字段定义 ==> Name *string `json:"name" db:"name"` // 名称
	StructColumnUpdate             string   // 表结构体字段更新 ==> if s.Id != t.Id { tmp["id"] = t.Id }

	StructColumnHeyValues          []string // NewHey.Attribute ==> Name:"name", // 名称
	StructColumnHeyValuesAccess    string   // NewHey.Access ==> Access:[]string{}, // 访问字段列表
	StructColumnHeyValuesAccessMap string   // NewHey.AccessMap ==> Access:map[string]struct{}, // 访问字段列表

	ColumnAutoIncr  string // 结构体字段方法 ColumnAutoIncr
	ColumnCreatedAt string // 结构体字段方法 ColumnCreatedAt
	ColumnUpdatedAt string // 结构体字段方法 ColumnUpdatedAt
	ColumnDeletedAt string // 结构体字段方法 ColumnDeletedAt
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

	// hey
	for i, c := range s.table.Column {
		tmp := fmt.Sprintf("\t%s string", c.pascal())
		comment := c.comment()
		if comment != "" {
			tmp = fmt.Sprintf("%s // %s", tmp, comment)
		}
		if i != 0 {
			tmp = fmt.Sprintf("\n%s", tmp)
		}
		s.StructColumnHey = append(s.StructColumnHey, tmp)
	}

	// column list
	{
		lengthColumn := len(s.table.Column)
		field := make([]string, 0, lengthColumn)
		fieldAccess := make([]string, 0, lengthColumn)
		for i, c := range s.table.Column {
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
			s.StructColumnHeyValues = append(s.StructColumnHeyValues, tmp)
		}

		{
			s96 := string(byte96) // `
			s34 := `"`            // "
			s.StructColumnHeyFieldSlice = strings.Join(field, ", ")
			switch s.table.app.TypeDriver() {
			case DriverMysql:
				s.StructColumnHeyFieldSlice = strings.ReplaceAll(s.StructColumnHeyFieldSlice, s34, s96)
			}
			if strings.Contains(s.StructColumnHeyFieldSlice, s96) {
				s.StructColumnHeyFieldSliceValue = hey.ConcatString(s34, s.StructColumnHeyFieldSlice, s34)
			} else {
				s.StructColumnHeyFieldSliceValue = hey.ConcatString(s96, s.StructColumnHeyFieldSlice, s96)
			}
		}

		s.StructColumnHeyValuesAccess = fmt.Sprintf("[]string{\n\t\t%s\n\t}", strings.Join(fieldAccess, "\n\t\t"))

		fieldAccessMap := fieldAccess[:]
		for k, v := range fieldAccessMap {
			fieldAccessMap[k] = strings.Replace(v, ",", ":{},", 1)
		}
		s.StructColumnHeyValuesAccessMap = fmt.Sprintf("map[string]*struct{}{\n\t\t%s\n\t}", strings.Join(fieldAccessMap, "\n\t\t"))
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
		autoIncrement := fc(s.table.app.FieldSerial) // auto increment column
		if s.table.TableFieldSerial != "" && s.table.app.FieldSerial != s.table.TableFieldSerial {
			autoIncrement = append(autoIncrement, s.table.TableFieldSerial)
		}
		created := fc(strings.Split(s.table.app.FieldCreatedAt, ",")...) // created_at columns
		updated := fc(strings.Split(s.table.app.FieldUpdatedAt, ",")...) // updated_at columns
		deleted := fc(strings.Split(s.table.app.FieldDeletedAt, ",")...) // deleted_at columns

		ignore = append(ignore, autoIncrement[:]...)
		ignore = append(ignore, created[:]...)

		for _, field := range ignore {
			cannotBeUpdatedFieldsMap[field] = &struct{}{}
		}

		ignore = append(ignore, updated[:]...)
		ignore = append(ignore, deleted[:]...)

		if len(autoIncrement) > 0 && autoIncrement[0] != "" {
			s.ColumnAutoIncr = fmt.Sprintf("[]string{ s.%s }", pascal(autoIncrement[0]))
		} else {
			s.ColumnAutoIncr = "nil"
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
		s.ColumnCreatedAt = cs(created...)
		s.ColumnUpdatedAt = cs(updated...)
		s.ColumnDeletedAt = cs(deleted...)
	}

	// update column
	columnUpdates := make([]string, 0)
	for _, c := range s.table.Column {
		o := *c.ColumnName
		if _, ok := cannotBeUpdatedFieldsMap[o]; ok {
			continue
		}
		p := c.pascal()
		update := fmt.Sprintf(`
	if s.%s != c.%s {
		tmp["%s"] = c.%s
	}`, p, p, o, p)
		columnUpdates = append(columnUpdates, update)
	}
	s.StructColumnUpdate = strings.Join(columnUpdates, "")

	ignoreMap := make(map[string]struct{})
	for _, v := range ignore {
		ignoreMap[v] = struct{}{}
	}

	// add
	write := false
	for _, c := range s.table.Column {
		if _, ok := ignoreMap[*c.ColumnName]; ok {
			continue // ignore columns like id, created_at, updated_at, deleted_at
		}
		if s.table.TableFieldSerial == *c.ColumnName {
			continue
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
			tmp = fmt.Sprintf("\t%s %s `json:\"%s\" db:\"-\" validate:\"omitempty\"`",
				c.pascal(),
				c.databaseTypeToGoType(),
				underline(*c.ColumnName),
			)
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

}

func (s *App) Model() error {
	// model.go
	if err := s.MakeTmplWire("model"); err != nil {
		return err
	}
	// model_schema.go
	tmpModelSchema := NewTemplate("tmpl_model_schema", tmplModelSchema)
	tmpModelSchemaContent := NewTemplate("tmpl_model_schema_content", tmplModelSchemaContent)
	modelSchemaFilename := pathJoin(s.OutputDirectory, "model", "model_schema.go")
	modelSchemaBuffer := bytes.NewBuffer(nil)
	modelTableCreateFilename := pathJoin(s.OutputDirectory, "model", "table_create.sql")
	modelTableCreateBuffer := bytes.NewBuffer(nil)
	for _, table := range s.ber.AllTable() {
		// for table ddl
		{
			ddl := table.DDL
			for strings.HasSuffix(ddl, "\n") {
				ddl = strings.TrimSuffix(ddl, "\n")
			}
			if _, err := modelTableCreateBuffer.WriteString(ddl); err != nil {
				return err
			}
			if !strings.HasSuffix(ddl, ";") {
				if _, err := modelTableCreateBuffer.WriteString(";"); err != nil {
					return err
				}
			}
			if _, err := modelTableCreateBuffer.WriteString("\n"); err != nil {
				return err
			}
		}
		// table
		modelSchemaContentBuffer := bytes.NewBuffer(nil)
		tmp := table.TmplTableModel()
		tmp.Make()
		if err := tmpModelSchemaContent.Execute(modelSchemaContentBuffer, tmp); err != nil {
			return err
		}
		modelSchemaContentFilename := pathJoin(s.OutputDirectory, "model", fmt.Sprintf("%s%s%s%s", tableFilenamePrefix, *table.TableName, tableFilenameSuffix, tableFilenameGo))
		if err := s.WriteFile(modelSchemaContentBuffer, modelSchemaContentFilename); err != nil {
			return err
		}

	}
	// model_schema.go
	if err := tmpModelSchema.Execute(modelSchemaBuffer, s); err != nil {
		return err
	}
	if err := s.WriteFile(modelSchemaBuffer, modelSchemaFilename); err != nil {
		return err
	}
	// table_create.sql
	if err := s.WriteFile(modelTableCreateBuffer, modelTableCreateFilename); err != nil {
		return err
	}
	return nil
}

type TmplTableDataSchema struct {
	Version string // 模板版本

	PrefixPackage string // 包导入前缀

	// data
	MapListDefine  string // data_schema.go tables define
	MapListParams  string // data_schema.go tables params
	MapListAssign  string // data_schema.go tables assign
	MapListStorage string // data_schema.go tables storage
	MapListSlice   string // data_schema.go tables slice
}

type TmplTableDataSchemaAab struct {
	Version string // 模板版本

	OriginName           string // 原始表名称
	OriginNamePascal     string // 原始表名称(帕斯卡命名)
	OriginNameWithPrefix string // 原始表名称
	OriginNameCamel      string // 表名(帕斯卡命名)首字母小写表名
	Comment              string // 表注释(如果表没有注释使用原始表名作为默认值)

	PrefixPackage string // 包导入前缀
}

type TmplTableData struct {
	table *SysTable

	Version string // 模板版本

	OriginName           string // 原始表名称
	OriginNamePascal     string // 原始表名称(帕斯卡命名)
	OriginNameWithPrefix string // 原始表名称
	OriginNameCamel      string // 表名(帕斯卡命名)首字母小写表名
	Comment              string // 表注释(如果表没有注释使用原始表名作为默认值)

	PrefixPackage string // 包导入前缀
}

type TmplTableAbcSchema struct {
	Version string // 模板版本

	PrefixPackage string // 包导入前缀
}

func (s *App) Data() error {
	// data.go
	if err := s.MakeTmplWire(
		"data",
		"\n\tNewTables, // all instances",
	); err != nil {
		return err
	}
	// data_schema.go
	tmpDataSchema := NewTemplate("data_schema", tmplDataSchema)
	tmpDataSchemaContent := NewTemplate("data_schema_content", tmplDataSchemaContent)
	tmpDataSchemaContentAab := NewTemplate("data_schema_content_aab", tmplDataSchemaContentAab)
	tables := s.ber.AllTable()
	schema := &TmplTableDataSchema{
		Version:       s.Version,
		PrefixPackage: s.PrefixPackageName,
	}
	for _, table := range tables {
		tmp := table.TmplTableData()
		schemaContentBuffer := bytes.NewBuffer(nil)
		if err := tmpDataSchemaContent.Execute(schemaContentBuffer, tmp); err != nil {
			return err
		}
		schemaContentFilename := pathJoin(s.OutputDirectory, "data", fmt.Sprintf("%s%s%s%s", tableFilenamePrefix, *table.TableName, tableFilenameSuffix, tableFilenameGo))
		if err := s.WriteFile(schemaContentBuffer, schemaContentFilename); err != nil {
			return err
		}

		schemaContentFilenameAab := pathJoin(s.OutputDirectory, "data", fmt.Sprintf("%s%s%s%s", tableFilenamePrefix, *table.TableName, tableFilenameSuffix1, tableFilenameGo))
		if _, err := os.Stat(schemaContentFilenameAab); err == nil {
			schemaContentFilenameAab = tableFilenameGoTmp(schemaContentFilenameAab)
		}
		tmpAab := &TmplTableDataSchemaAab{
			Version: tmp.Version,

			OriginName:           tmp.OriginName,
			OriginNamePascal:     tmp.OriginNamePascal,
			OriginNameWithPrefix: tmp.OriginNameWithPrefix,
			OriginNameCamel:      tmp.OriginNameCamel,
			Comment:              tmp.Comment,

			PrefixPackage: tmp.PrefixPackage,
		}
		schemaContentBufferAab := bytes.NewBuffer(nil)
		if err := tmpDataSchemaContentAab.Execute(schemaContentBufferAab, tmpAab); err != nil {
			return err
		}
		if err := s.WriteFile(schemaContentBufferAab, schemaContentFilenameAab); err != nil {
			return err
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
	schema.MapListDefine = strings.Join(defines, "\n\t")
	schema.MapListParams = strings.Join(params, "\n\t")
	schema.MapListAssign = strings.Join(assigns, "\n\t\t")
	schema.MapListStorage = strings.Join(storage, "\n\t\t")
	schema.MapListSlice = strings.Join(slice, "\n\t\t")
	if err := tmpDataSchema.Execute(buffer, schema); err != nil {
		return err
	}
	if err := s.WriteFile(buffer, filename); err != nil {
		return err
	}

	if s.Admin || s.Index {
		// abc_schema.go
		abcSchema := &TmplTableAbcSchema{
			Version:       s.Version,
			PrefixPackage: s.PrefixPackageName,
		}
		tmpAbcSchema := NewTemplate("abc_schema", tmplAbcSchema)
		abcFilename := pathJoin(s.OutputDirectory, "abc", "abc_schema.go")
		abcBuffer := bytes.NewBuffer(nil)
		if err := tmpAbcSchema.Execute(abcBuffer, abcSchema); err != nil {
			return err
		}
		if err := s.WriteFile(abcBuffer, abcFilename); err != nil {
			return err
		}
	}

	return nil
}

type TmplTableBizSchema struct {
	Version string // 模板版本

	PrefixPackage string // 包导入前缀

	// biz
	AllTablesSchemaContent string // biz.go data all tables schema content
}

type TmplTableBiz struct {
	table *SysTable

	Version string // 模板版本

	OriginName           string // 原始表名称
	OriginNamePascal     string // 原始表名称(帕斯卡命名)
	OriginNameWithPrefix string // 原始表名称
	OriginNameCamel      string // 表名(帕斯卡命名)首字母小写表名
	Comment              string // 表注释(如果表没有注释使用原始表名作为默认值)

	PrefixPackage string // 包导入前缀
}

func (s *App) Biz() error {
	// biz.go
	if err := s.MakeTmplWire("biz"); err != nil {
		return err
	}
	// biz_schema.tmpl
	tmpBizSchema := NewTemplate("biz_schema", tmplBizSchema)
	tmpBizSchemaContent := NewTemplate("biz_schema_content", tmplBizSchemaContent)
	buffer := bytes.NewBuffer(nil)
	schema := &TmplTableBizSchema{
		Version:       s.Version,
		PrefixPackage: s.PrefixPackageName,
	}
	for _, table := range s.ber.AllTable() {
		tmp := table.TmplTableBiz()
		if err := tmpBizSchemaContent.Execute(buffer, tmp); err != nil {
			return err
		}
	}
	filename := pathJoin(s.OutputDirectory, "biz", "biz_schema.tmp")
	bizSchemaBuffer := bytes.NewBuffer(nil)
	schema.AllTablesSchemaContent = buffer.String()
	if err := tmpBizSchema.Execute(bizSchemaBuffer, schema); err != nil {
		return err
	}
	if err := s.WriteFile(bizSchemaBuffer, filename); err != nil {
		return err
	}
	return nil
}

type TmplTableAsc struct {
	table *SysTable

	Version string // 模板版本

	OriginName           string // 原始表名称
	OriginNamePascal     string // 原始表名称(帕斯卡命名)
	OriginNameWithPrefix string // 原始表名称
	OriginNameCamel      string // 表名(帕斯卡命名)首字母小写表名
	Comment              string // 表注释(如果表没有注释使用原始表名作为默认值)

	PrefixPackage string // 包导入前缀

	UrlPrefix string // 路由前缀

	PseudoDelete string // 移动到回收站功能(伪删除)

	CustomMethod string // asc custom methods

	FileNamePrefix string // 文件名前缀
}

type TmplTableAscPseudoDelete struct {
	UrlPrefix       string // 路由前缀
	TableName       string
	TableNamePascal string
	TableComment    string
	FieldLists      []string
}

func (s *TmplTableAsc) Make() (buffer *bytes.Buffer, err error) {
	buffer = bytes.NewBuffer(nil)
	if s.table.app.FieldDeletedAt == "" {
		return
	}
	if s.table.TableFieldSerial != hey.Id {
		return
	}
	fields := strings.Split(s.table.app.FieldDeletedAt, ",")
	splits := make([]string, 0, len(fields))
	for _, v := range fields {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		for _, c := range s.table.Column {
			if c.ColumnName == nil || *c.ColumnName != v {
				continue
			}
			// splits = append(splits, v)
			// continue
			t := c.databaseTypeToGoType()
			t = strings.ReplaceAll(t, "*", "")
			t = strings.TrimSpace(t)
			switch t {
			case "int", "int64":
				splits = append(splits, v)
			}
		}
	}
	if len(splits) == 0 {
		return
	}
	value := &TmplTableAscPseudoDelete{
		UrlPrefix:       s.UrlPrefix,
		TableName:       s.OriginName,
		TableNamePascal: s.OriginNamePascal,
		TableComment:    s.Comment,
		FieldLists:      splits,
	}
	s.PseudoDelete = `r.DELETE("", s.Delete)`
	temp := NewTemplate("asc_schema_content_custom", tmplAscSchemaContentCustom)
	if err = temp.Execute(buffer, value); err != nil {
		return
	}
	return
}

func (s *App) Asc() error {
	// asc.go
	if err := s.MakeTmplWire("asc"); err != nil {
		return err
	}
	tmpAscSchemaContent := NewTemplate("asc_schema_content", tmplAscSchemaContent)
	tables := s.ber.AllTable()
	for _, table := range tables {
		tmp := table.TmplTableAsc()
		tmp.UrlPrefix = s.AdminUrlPrefix

		schemaContentBuffer := bytes.NewBuffer(nil)
		customMethodBuffer, err := tmp.Make()
		if err != nil {
			return err
		}

		// 自定义方法列表(伪删除)
		tmp.CustomMethod = customMethodBuffer.String()
		if err = tmpAscSchemaContent.Execute(schemaContentBuffer, tmp); err != nil {
			return err
		}

		// PREFIX_table_name_SUFFIX.go
		schemaContentFilename := pathJoin(s.OutputDirectory, "asc", fmt.Sprintf("%s%s%s%s", tableFilenamePrefix, *table.TableName, tableFilenameSuffix, tableFilenameGo))
		if _, err = os.Stat(schemaContentFilename); err == nil {
			schemaContentFilename = tableFilenameGoTmp(schemaContentFilename)
		}
		if err = s.WriteFile(schemaContentBuffer, schemaContentFilename); err != nil {
			return err
		}

		// PREFIX_table_name_SUFFIX.go 自定义业务代码
		tmpAscSchemaContentBusinessGo := NewTemplate("asc_schema_content_aab", tempAscSchemaCustomAab)
		tmpAscSchemaContentBusinessGoBuffer := bytes.NewBuffer(nil)
		if err = tmpAscSchemaContentBusinessGo.Execute(tmpAscSchemaContentBusinessGoBuffer, tmp); err != nil {
			return err
		}
		schemaCustomAabFilename := pathJoin(s.OutputDirectory, "asc", fmt.Sprintf("%s%s%s%s", tableFilenamePrefix, *table.TableName, tableFilenameSuffix1, tableFilenameGo))
		if _, err = os.Stat(schemaCustomAabFilename); err == nil {
			schemaCustomAabFilename = tableFilenameGoTmp(schemaCustomAabFilename)
		}
		if err = s.WriteFile(tmpAscSchemaContentBusinessGoBuffer, schemaCustomAabFilename); err != nil {
			return err
		}
	}

	return nil
}

type TmplTableCanSchema struct {
	Version string // 模板版本

	PrefixPackage string // 包导入前缀

	// can
	AutoRegisterRouterParams     string // can_schema.go tables params
	AutoRegisterRouterParamsCopy string // can_schema.go tables params with this package name
	AutoRegisterRouterSlice      string // can_schema.go tables slice
}

type TmplTableCan struct {
	table *SysTable

	Version string // 模板版本

	OriginName           string // 原始表名称
	OriginNamePascal     string // 原始表名称(帕斯卡命名)
	OriginNameWithPrefix string // 原始表名称
	OriginNameCamel      string // 表名(帕斯卡命名)首字母小写表名
	Comment              string // 表注释(如果表没有注释使用原始表名作为默认值)

	PrefixPackage string // 包导入前缀

	UrlPrefix string // 路由前缀

	FileNamePrefix string // 文件名前缀
}

func (s *App) Can() error {
	// can.go
	if err := s.MakeTmplWire("can"); err != nil {
		return err
	}
	// can_schema.go
	tmpCanSchema := NewTemplate("can_schema", tmplCanSchema)
	tmpCanSchemaContent := NewTemplate("can_schema_content", tmplCanSchemaContent)
	schema := &TmplTableCanSchema{
		Version:       s.Version,
		PrefixPackage: s.PrefixPackageName,
	}
	tables := s.ber.AllTable()
	for _, table := range tables {
		tmp := table.TmplTableCan()
		tmp.UrlPrefix = s.IndexUrlPrefix

		schemaContentBuffer := bytes.NewBuffer(nil)

		if err := tmpCanSchemaContent.Execute(schemaContentBuffer, tmp); err != nil {
			return err
		}

		// PREFIX_table_name_SUFFIX.go
		schemaContentFilename := pathJoin(s.OutputDirectory, "can", fmt.Sprintf("%s%s%s%s", tableFilenamePrefix, *table.TableName, tableFilenameSuffix, tableFilenameGo))
		if _, err := os.Stat(schemaContentFilename); err == nil {
			schemaContentFilename = tableFilenameGoTmp(schemaContentFilename)
		}
		if err := s.WriteFile(schemaContentBuffer, schemaContentFilename); err != nil {
			return err
		}
	}
	filename := pathJoin(s.OutputDirectory, "can", "can_schema.go")
	buffer := bytes.NewBuffer(nil)
	length := len(tables)
	params := make([]string, 0, length)
	paramsCopy := make([]string, 0, length)
	slice := make([]string, 0, length)
	for _, table := range tables {
		namePascal := table.pascal()
		namePascalSmall := table.pascalSmall()
		params = append(params, fmt.Sprintf("%s *%s,", namePascalSmall, namePascal))
		paramsCopy = append(paramsCopy, fmt.Sprintf("// %s *can.%s,", namePascalSmall, namePascal))
		slice = append(slice, fmt.Sprintf("%s,", namePascalSmall))
	}
	schema.AutoRegisterRouterParams = strings.Join(params, "\n\t")
	schema.AutoRegisterRouterParamsCopy = strings.Join(paramsCopy, "\n")
	schema.AutoRegisterRouterSlice = strings.Join(slice, "\n\t\t")
	if err := tmpCanSchema.Execute(buffer, schema); err != nil {
		return err
	}
	if err := s.WriteFile(buffer, filename); err != nil {
		return err
	}

	return nil
}
