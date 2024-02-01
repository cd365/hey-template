package main

import (
	"flag"
	"fmt"
	"main/dbs"
)

var param = &dbs.Param{}

func init() {
	flag.StringVar(&param.Driver, "d", "postgres", "driver name: mysql postgres")                                                                                      // driver name
	flag.StringVar(&param.DataSourceName, "dsn", "root:112233@tcp(127.0.0.1:3306)/hello?charset=utf8mb4&collation=utf8mb4_unicode_ci&timeout=90s", "data source name") // data source name
	flag.StringVar(&param.DatabaseSchemaName, "dos", "", "database or schema name")                                                                                    // database or schema name
	flag.StringVar(&param.OutputDirectory, "o", "", "output directory")                                                                                                // output directory
	flag.StringVar(&param.ImportModelPackageName, "mp", "main/model", "model package name")                                                                            // model package name
	flag.BoolVar(&param.UsingDatabaseSchemaName, "p", false, "use database or schema name as table name prefix")                                                       // use database or schema name as table name prefix
	flag.StringVar(&param.FieldsAutoIncrement, "aic", "id", "auto increment column")                                                                                   // auto increment column
	flag.StringVar(&param.FieldsListCreatedAt, "add", "created_at,add_at", "automatically set timestamp on insert, int type")                                          // created at
	flag.StringVar(&param.FieldsListUpdatedAt, "mod", "updated_at,mod_at", "automatically set timestamp on update, int type")                                          // updated at
	flag.StringVar(&param.FieldsListDeletedAt, "del", "deleted_at", "delete timestamp, int type")                                                                      // deleted at
	flag.Parse()
}

func main() {
	if err := param.BuildAll(); err != nil {
		fmt.Println(err.Error())
	}
}
