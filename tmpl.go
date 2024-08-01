package main

import (
	_ "embed"
)

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

	//go:embed tmpl/data_schema_content123.tmpl
	tmplDataSchemaContent123 []byte

	//go:embed tmpl/biz_schema.tmpl
	tmplBizSchema []byte

	//go:embed tmpl/biz_schema_content.tmpl
	tmplBizSchemaContent []byte

	//go:embed tmpl/abc_schema.tmpl
	tmplAbcSchema []byte

	//go:embed tmpl/asc_schema_content.tmpl
	tmplAscSchemaContent []byte

	//go:embed tmpl/asc_schema_content_custom.tmpl
	tmplAscSchemaContentCustom []byte

	//go:embed tmpl/asc_schema_content123.tmpl
	tempAscSchemaCustom123 []byte

	//go:embed tmpl/can_schema.tmpl
	tmplCanSchema []byte

	//go:embed tmpl/can_schema_content.tmpl
	tmplCanSchemaContent []byte

	//go:embed pgsql/func_create.sql
	pgsqlFuncCreate string

	//go:embed pgsql/func_drop.sql
	pgsqlFuncDrop string
)
