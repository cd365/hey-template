package app

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"regexp"
	"root/utils"
	"strings"
)

type Config struct {
	Version  string `json:"version" yaml:"version"`     // 构建版本
	BuildAt  string `json:"build_at" yaml:"build_at"`   // 构建时间
	CommitId string `json:"commit_id" yaml:"commit_id"` // GIT提交HASH值

	SchemaId string `json:"schema_id" yaml:"schema_id"` // 模板代码中的schema unique value

	Driver         string `json:"driver" yaml:"driver"`                     // 数据库驱动名称 mysql|postgres
	DataSourceName string `json:"data_source_name" yaml:"data_source_name"` // 数据源地址 mysql=>root:112233@tcp(127.0.0.1:3306)/hello?charset=utf8mb4&collation=utf8mb4_unicode_ci&timeout=90s pgsql=>postgres://postgres:112233@[::1]:5432/hello?sslmode=disable

	TableSchemaName      string `json:"table_schema_name" yaml:"table_schema_name"`             // 数据库模式名称 mysql可以使用数据库名,pgsql可以使用schema名称 mysql默认空,pgsql默认public
	UsingTableSchemaName bool   `json:"using_table_schema_name" yaml:"using_table_schema_name"` // 是否使用模式名称 在表名之前指定模式名称 如: public.account

	ColumnSerial    string `json:"column_serial" yaml:"column_serial"`         // 表的序号字段(自动递增的字段) 数据库表本身应该具有唯一字段名 只能设置一个字段 通常是 id
	ColumnCreatedAt string `json:"column_created_at" yaml:"column_created_at"` // 表数据创建时间标记字段 通常是int或者int64类型 多个使用','隔开
	ColumnUpdatedAt string `json:"column_updated_at" yaml:"column_updated_at"` // 表数据更新时间标记字段 通常是int或者int64类型 多个使用','隔开
	ColumnDeletedAt string `json:"column_deleted_at" yaml:"column_deleted_at"` // 表数据伪删除时间标记字段 通常是int或者int64类型 多个使用','隔开

	ImportPrefixPackageName string `json:"import_prefix_package_name" yaml:"import_prefix_package_name"` // 导入包名前缀 默认main
	TemplateOutputDirectory string `json:"template_output_directory" yaml:"template_output_directory"`   // 模板文件输出路径

	DisableTableNameMatchRules []string         `json:"disable_table_name_match_rules" yaml:"disable_table_name_match_rules"` // 禁止构建表的正则表达式 表名称只需要满足其中一条正则表达式即可 不配置即不限制
	disableTableNameMatchRules []*regexp.Regexp // 禁止构建表的正则表达式 表名称只需要满足其中一条正则表达式即可 不配置即不限制

	AllowTableName           []string         `json:"allow_table_name" yaml:"allow_table_name"`                         // 满足禁止构建中的某一条正则,但是又需要使用该表的情况,用于设置特定的表名 (优先级高于 DisableTableNameMatchRules)
	AllowTableNameMatchRules []string         `json:"allow_table_name_match_rules" yaml:"allow_table_name_match_rules"` // 满足禁止构建中的某一条正则,但是又满足当前允许构建中的某一条正则 (优先级高于 DisableTableNameMatchRules)
	allowTableNameMatchRules []*regexp.Regexp // 允许构建表的正则表达式 表名称只需要满足其中一条正则表达式即可 不配置即无效 (AllowTableName 和 AllowTableNameMatchRules 可搭配使用, AllowTableName 优先使用)

	DatabaseIdentify string `json:"-" yaml:"-"` // 数据库标识符号 mysql: ` postgres: "
}

func (s *Config) Initial() error {
	for _, v := range s.DisableTableNameMatchRules {
		tmpRegexp, err := regexp.Compile(v)
		if err != nil {
			return err
		}
		s.disableTableNameMatchRules = append(s.disableTableNameMatchRules, tmpRegexp)
	}
	for _, v := range s.AllowTableNameMatchRules {
		tmpRegexp, err := regexp.Compile(v)
		if err != nil {
			return err
		}
		s.allowTableNameMatchRules = append(s.allowTableNameMatchRules, tmpRegexp)
	}
	return nil
}

func (s *Config) Disable(table string) bool {
	if s.disableTableNameMatchRules == nil {
		return false
	}
	for _, disable := range s.disableTableNameMatchRules {
		if disable.MatchString(table) {
			for _, allow := range s.AllowTableName {
				if allow == table {
					return false
				}
			}
			for _, allow := range s.allowTableNameMatchRules {
				if allow.MatchString(table) {
					return false
				}
			}
			return true
		}
	}
	return false
}

func (s *Config) SchemaValue() string {
	prefix := "Schema"
	if s.SchemaId != "" {
		if strings.HasPrefix(s.SchemaId, prefix) {
			return s.SchemaId
		}
		return fmt.Sprintf("%s%s", prefix, s.SchemaId)
	}
	return fmt.Sprintf("%s%s", prefix, utils.RandomString(9))
}

// InitConfig 初始化默认配置
func InitConfig(configFile string) error {
	fil, err := os.OpenFile(configFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer func() { _ = fil.Close() }()
	config := &Config{
		Version:                 "v0.0.1",
		BuildAt:                 "20200303080000",
		CommitId:                "0000000000000000000000000000000000000000",
		Driver:                  "postgres",
		DataSourceName:          "postgres://postgres:112233@[::1]:5432/hello?sslmode=disable",
		TableSchemaName:         "public",
		ImportPrefixPackageName: "main",
		UsingTableSchemaName:    true,
		ColumnSerial:            "id",
		ColumnCreatedAt:         "created_at,add_at",
		ColumnUpdatedAt:         "updated_at,mod_at",
		ColumnDeletedAt:         "deleted_at,del_at",
		TemplateOutputDirectory: "",
		DisableTableNameMatchRules: []string{
			"^aaa_.*$",
			"^zzz_.*$",
		},
		DatabaseIdentify: "\"",
	}
	encoder := yaml.NewEncoder(fil)
	if err = encoder.Encode(config); err != nil {
		return err
	}
	return nil
}

func ReadConfig(configFile string) (*Config, error) {
	stat, err := os.Stat(configFile)
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		return nil, fmt.Errorf("config file is a directory")
	}
	fil, err := os.OpenFile(configFile, os.O_RDONLY, 0644)
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
