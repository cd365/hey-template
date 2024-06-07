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

	//go:embed tmpl/data_schema_content_custom.tmpl
	tmplDataSchemaContentCustom []byte

	//go:embed tmpl/biz_schema.tmpl
	tmplBizSchema []byte

	//go:embed tmpl/biz_schema_content.tmpl
	tmplBizSchemaContent []byte

	//go:embed tmpl/arm_schema.tmpl
	tmplArmSchema []byte

	//go:embed tmpl/arm_schema_content.tmpl
	tmplArmSchemaContent []byte

	//go:embed tmpl/arm_schema_content_custom.tmpl
	tmplArmSchemaContentCustom []byte

	//go:embed tmpl/arm_schema_content_aaa.tmpl
	tempArmSchemaCustomAaa []byte

	//go:embed pgsql/func_create.sql
	pgsqlFuncCreate string

	//go:embed pgsql/func_drop.sql
	pgsqlFuncDrop string
)
