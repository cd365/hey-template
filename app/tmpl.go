package app

import (
	_ "embed"
)

var (
	//go:embed tmpl/model_schema.tmpl
	tmplModelSchema []byte

	//go:embed tmpl/model_schema_custom.tmpl
	tmplModelSchemaCustom []byte

	//go:embed tmpl/model_schema_content.tmpl
	tmplModelSchemaContent []byte

	//go:embed tmpl/model_schema_content_primary_key.tmpl
	tmplModelSchemaContentPrimaryKey []byte

	//go:embed tmpl/pgsql/func_create.sql
	pgsqlFuncCreate string

	//go:embed tmpl/pgsql/func_drop.sql
	pgsqlFuncDrop string
)
