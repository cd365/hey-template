package main

import (
	_ "embed"
)

var (
	//go:embed tmpl/wire.tmpl
	tmplWire []byte

	//go:embed tmpl/model_schema.tmpl
	tmplModelSchema []byte

	//go:embed tmpl/model_schema_custom.tmpl
	tmplModelSchemaCustom []byte

	//go:embed tmpl/model_schema_content.tmpl
	tmplModelSchemaContent []byte

	//go:embed pgsql/func_create.sql
	pgsqlFuncCreate string

	//go:embed pgsql/func_drop.sql
	pgsqlFuncDrop string
)
