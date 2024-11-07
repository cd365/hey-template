package main

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

type Config struct {
	Version string `json:"version" yaml:"version"`   // 构建版本
	BuildAt string `json:"build_at" yaml:"build_at"` // 构建时间
	GitHash string `json:"git_hash" yaml:"git_hash"` // GIT提交HASH值

	Driver         string `json:"driver" yaml:"driver"`                     // 数据库驱动名称 mysql|postgres
	DataSourceName string `json:"data_source_name" yaml:"data_source_name"` // 数据源地址 mysql=>root:112233@tcp(127.0.0.1:3306)/hello?charset=utf8mb4&collation=utf8mb4_unicode_ci&timeout=90s pgsql=>postgres://postgres:112233@[::1]:5432/hello?sslmode=disable

	TableSchemaName         string `json:"table_schema_name" yaml:"table_schema_name"`                   // 数据库模式名称 mysql可以使用数据库名,pgsql可以使用schema名称 mysql默认空,pgsql默认public
	ImportPrefixPackageName string `json:"import_prefix_package_name" yaml:"import_prefix_package_name"` // 导入包名前缀 默认main
	UsingTableSchemaName    bool   `json:"use_table_schema_name" yaml:"use_table_schema_name"`           // 是否使用模式名称 在表名之前指定模式名称 如: public.account

	ColumnSerial    string `json:"column_serial" yaml:"column_serial"`         // 表的序号字段(自动递增的字段) 数据库表本身应该具有唯一字段名 只能设置一个字段 通常是 id
	ColumnCreatedAt string `json:"column_created_at" yaml:"column_created_at"` // 表数据创建时间标记字段 通常是int或者int64类型 多个使用','隔开
	ColumnUpdatedAt string `json:"column_updated_at" yaml:"column_updated_at"` // 表数据更新时间标记字段 通常是int或者int64类型 多个使用','隔开
	ColumnDeletedAt string `json:"column_deleted_at" yaml:"column_deleted_at"` // 表数据伪删除时间标记字段 通常是int或者int64类型 多个使用','隔开

	TemplateOutputDirectory string `json:"template_output_directory" yaml:"template_output_directory"` // 模板文件输出路径

	AllowTableNameMatchRules   []string `json:"allow_table_name_match_rules" yaml:"allow_table_name_match_rules"`     // 允许构建表的正则表达式 表名称只需要满足其中一条正则表达式即可 不配置即不限制
	DisableTableNameMatchRules []string `json:"disable_table_name_match_rules" yaml:"disable_table_name_match_rules"` // 禁止构建表的正则表达式 表名称只需要满足其中一条正则表达式即可 不配置即不限制

	DatabaseIdentify string `json:"-" yaml:"-"` // 数据库标识符号 mysql: ` postgres: "
}

func SetConfig(configFile string) error {
	fil, err := os.OpenFile(configFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer func() { _ = fil.Close() }()
	config := &Config{
		Version:                    "v0.0.1",
		BuildAt:                    "",
		GitHash:                    "",
		Driver:                     "postgres",
		DataSourceName:             "postgres://postgres:112233@[::1]:5432/hello?sslmode=disable",
		TableSchemaName:            "public",
		ImportPrefixPackageName:    "main",
		UsingTableSchemaName:       true,
		ColumnSerial:               "id",
		ColumnCreatedAt:            "created_at,add_at",
		ColumnUpdatedAt:            "updated_at,mod_at",
		ColumnDeletedAt:            "deleted_at,del_at",
		TemplateOutputDirectory:    "",
		AllowTableNameMatchRules:   nil,
		DisableTableNameMatchRules: nil,
		DatabaseIdentify:           "\"",
	}
	encoder := yaml.NewEncoder(fil)
	if err = encoder.Encode(config); err != nil {
		return err
	}
	return nil
}

func GetConfig(configFile string) (*Config, error) {
	stat, err := os.Stat(configFile)
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		return nil, fmt.Errorf("config file is a directory")
	}
	fil, err := os.OpenFile(configFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}
	defer func() { _ = fil.Close() }()
	config := &Config{}
	if err = yaml.NewDecoder(fil).Decode(config); err != nil {
		return nil, err
	}
	return config, nil
}
