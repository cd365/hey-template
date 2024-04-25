package main

import (
	"flag"
	"fmt"
)

func main() {
	flag.StringVar(&cmd.Driver, "d", "postgres", "driver name: mysql postgres")                                                         // driver name
	flag.StringVar(&cmd.DataSourceName, "dsn", "postgres://postgres:112233@localhost:5432/hello?sslmode=disable", "data source name")   // data source name; mysql=>root:112233@tcp(127.0.0.1:3306)/hello?charset=utf8mb4&collation=utf8mb4_unicode_ci&timeout=90s pgsql=>postgres://postgres:112233@[::1]:5432/hello?sslmode=disable
	flag.StringVar(&cmd.TablePrefixName, "s", "", "table prefix name or schema name")                                                   // table prefix name or schema name
	flag.StringVar(&cmd.OutputDirectory, "o", "", "output directory")                                                                   // output directory
	flag.StringVar(&cmd.PrefixPackageName, "pp", "main", "package prefix name")                                                         // package prefix name
	flag.BoolVar(&cmd.TablePrefix, "tp", false, "whether to use a table prefix")                                                        // whether to use a table prefix
	flag.StringVar(&cmd.TableMethodByField, "tmf", "table1.field1,table2.field2,table3.field3", "single field custom methods, in data") // single field custom methods, in data
	flag.StringVar(&cmd.FieldSerial, "fs", "id", "auto increment column")                                                               // auto increment column
	flag.StringVar(&cmd.FieldCreatedAt, "add", "created_at,add_at", "automatically set timestamp on insert, int type")                  // created at
	flag.StringVar(&cmd.FieldUpdatedAt, "mod", "updated_at,mod_at", "automatically set timestamp on update, int type")                  // updated at
	flag.StringVar(&cmd.FieldDeletedAt, "del", "deleted_at,del_at", "delete timestamp, int type")                                       // deleted at
	flag.BoolVar(&cmd.Admin, "admin", false, "quick insert, delete, update")                                                            // quick insert, delete, update
	flag.StringVar(&cmd.AdminUrlPrefix, "admin-url-prefix", "/api/v1/admin", "admin url prefix")                                        // admin url prefix
	flag.Parse()
	if err := cmd.BuildAll(); err != nil {
		fmt.Println(err.Error())
	}
}
