package initial

import (
	"context"
	"flag"
	"fmt"
	"os"
	"root/app"
	"root/values"
)

func Start() {

	configFile := ""
	printVersion := false

	flag.StringVar(&configFile, "c", "config.yaml", "yaml format config file") // yaml config file
	flag.BoolVar(&printVersion, "v", false, "view version")                    // view version

	flag.Parse()

	stat, err := os.Stat(configFile)
	if err != nil {
		if err = app.InitConfig(configFile); err != nil {
			fmt.Println("配置文件不存在,创建失败")
		}
		fmt.Println("配置文件不存在,现已创建")
		return
	}
	if stat.IsDir() {
		fmt.Println("配置文件是一个目录")
		return
	}

	cfg, err := app.ReadConfig(configFile)
	if err != nil {
		fmt.Println("解析配置文件失败", err.Error())
		return
	}
	if err = cfg.Initial(); err != nil {
		fmt.Println("配置文件初始化失败", err.Error())
		return
	}

	// 初始化
	sss, err := Init(context.Background(), cfg)
	if err != nil {
		fmt.Println("initial failed.", err.Error())
		return
	}

	{
		if cfg.BuildAt != "" {
			sss.Version = fmt.Sprintf("%s %s", sss.Version, cfg.BuildAt)
		} else {
			if values.BuildAt != "" {
				sss.Version = fmt.Sprintf("%s %s", sss.Version, values.BuildAt)
			}
		}
		if cfg.CommitId != "" {
			sss.Version = fmt.Sprintf("%s %s", sss.Version, cfg.CommitId)
		} else {
			if values.CommitId != "" {
				sss.Version = fmt.Sprintf("%s %s", sss.Version, values.CommitId)
			}
		}
	}

	if printVersion {
		fmt.Println(sss.Version)
		return
	}

	if err = sss.BuildAll(); err != nil {
		fmt.Println(err.Error())
	}

}
