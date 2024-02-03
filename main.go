package main

import (
	"flag"
	"fmt"
	"main/dbs"
)

const (
	Version = "v0.0.2"
)

var param = &dbs.Param{
	Version: Version,
}

func init() {
	flag.StringVar(&param.Driver, "d", "postgres", "driver name: mysql postgres")                                                       // driver name
	flag.StringVar(&param.DataSourceName, "dsn", "postgres://postgres:112233@localhost:5432/hello?sslmode=disable", "data source name") // data source name; mysql=>root:112233@tcp(127.0.0.1:3306)/hello?charset=utf8mb4&collation=utf8mb4_unicode_ci&timeout=90s pgsql=>postgres://postgres:112233@[::1]:5432/hello?sslmode=disable
	flag.StringVar(&param.DatabaseSchemaName, "dos", "", "database or schema name")                                                     // database or schema name
	flag.StringVar(&param.OutputDirectory, "o", "", "output directory")                                                                 // output directory
	flag.StringVar(&param.ImportModelPackageName, "mp", "main/model", "model package name")                                             // model package name
	flag.BoolVar(&param.UsingDatabaseSchemaName, "p", false, "use database or schema name as table name prefix")                        // use database or schema name as table name prefix
	flag.StringVar(&param.FieldsAutoIncrement, "aic", "id", "auto increment column")                                                    // auto increment column
	flag.StringVar(&param.FieldsListCreatedAt, "add", "created_at,add_at", "automatically set timestamp on insert, int type")           // created at
	flag.StringVar(&param.FieldsListUpdatedAt, "mod", "updated_at,mod_at", "automatically set timestamp on update, int type")           // updated at
	flag.StringVar(&param.FieldsListDeletedAt, "del", "deleted_at", "delete timestamp, int type")                                       // deleted at
	flag.Parse()
}

func main() {
	if err := param.BuildAll(); err != nil {
		fmt.Println(err.Error())
	}
}
