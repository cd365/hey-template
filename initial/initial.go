package initial

import (
	"context"
	"flag"
	"fmt"
	"github.com/cd365/hey-template/app"
	"github.com/cd365/hey-template/values"
	"os"
)

func Start() {

	configFile := ""
	printVersion := false

	// yaml config file
	flag.StringVar(&configFile, "c", "config.yaml", "yaml format config file")

	// view version
	flag.BoolVar(&printVersion, "v", false, "view version")

	flag.Parse()

	if printVersion {
		fmt.Println(values.Version, values.CommitId, values.BuildAt)
		return
	}

	{
		stat, err := os.Stat(configFile)
		if err != nil {
			if err = app.InitConfig(configFile); err != nil {
				fmt.Println("the configuration file does not exist, creation failed")
			}
			fmt.Println("the configuration file does not exist and has been created")
			return
		}
		if stat.IsDir() {
			fmt.Println("the configuration file is a directory")
			return
		}
	}

	cfg, err := app.ReadConfig(configFile)
	if err != nil {
		fmt.Println("failed to parse the configuration file", err.Error())
		return
	}
	if err = cfg.Initial(); err != nil {
		fmt.Println("configuration file initialization failed", err.Error())
		return
	}

	{
		cfg.Version = values.Version
		if cfg.CommitId != "" {
			cfg.Version = fmt.Sprintf("%s %s", cfg.Version, cfg.CommitId)
		} else {
			if values.CommitId != "" {
				cfg.Version = fmt.Sprintf("%s %s", cfg.Version, values.CommitId)
			}
		}
		if cfg.BuildAt != "" {
			cfg.Version = fmt.Sprintf("%s %s", cfg.Version, cfg.BuildAt)
		} else {
			if values.BuildAt != "" {
				cfg.Version = fmt.Sprintf("%s %s", cfg.Version, values.BuildAt)
			}
		}
		if cfg.Schema == "" {
			cfg.Schema = "S000001"
		}
	}

	sss, err := inject(context.Background(), cfg)
	if err != nil {
		fmt.Println("initial failed.", err.Error())
		return
	}

	if err = sss.BuildAll(); err != nil {
		fmt.Println(err.Error())
	}

}
