package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	viewVersion bool
	configFile  string
)

func main() {

	flag.StringVar(&configFile, "c", "config.yaml", "yaml format config file") // yaml config file
	flag.BoolVar(&viewVersion, "v", false, "view version")                     // view version
	flag.Parse()

	stat, err := os.Stat(configFile)
	if err != nil {
		if err = SetConfig(configFile); err != nil {
			fmt.Println("config file is not found, create failed.")
		}
		fmt.Println("config file is not found, create success.")
		return
	}
	if stat.IsDir() {
		fmt.Println("config file is directory")
		return
	}

	config, err := GetConfig(configFile)
	if err != nil {
		fmt.Println("parse config file failed.", err.Error())
		return
	}
	if err = config.Initial(); err != nil {
		fmt.Println("initialize config file failed.", err.Error())
		return
	}

	if config.BuildAt != "" {
		cmd.Version = fmt.Sprintf("%s %s", cmd.Version, config.BuildAt)
	} else {
		if BuildTime != "" {
			cmd.Version = fmt.Sprintf("%s %s", cmd.Version, BuildTime)
		}
	}

	if config.GitHash != "" {
		cmd.Version = fmt.Sprintf("%s %s", cmd.Version, config.GitHash)
	} else {
		if CommitHash != "" {
			cmd.Version = fmt.Sprintf("%s %s", cmd.Version, CommitHash)
		}
	}

	if viewVersion {
		fmt.Println(cmd.Version)
		return
	}

	cmd.cfg = config

	if err = cmd.BuildAll(); err != nil {
		fmt.Println(err.Error())
	}
}
