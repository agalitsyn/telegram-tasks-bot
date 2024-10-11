package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/agalitsyn/telegram-tasks-bot/pkg/flagtools"
	"github.com/agalitsyn/telegram-tasks-bot/pkg/secret"
	"github.com/agalitsyn/telegram-tasks-bot/pkg/version"

	"github.com/fatih/color"
	"github.com/go-pkgz/lgr"
)

const EnvPrefix = "TG_TASKS_BOT"

type Config struct {
	Debug bool
	Token secret.String
}

func (c Config) String() string {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stdout, err)
		os.Exit(0)
	}
	return string(b)
}

func ParseFlags() Config {
	var cfg Config

	flag.BoolVar(&cfg.Debug, "debug", false, "Debug mode.")
	token := flag.String("token", "", "Telegram bot token.")
	runPrintVersion := flag.Bool("version", false, "Show version.")

	flagtools.Prefix = EnvPrefix
	flagtools.Parse()
	flag.Parse()

	cfg.Token = secret.NewString(*token)

	if *runPrintVersion {
		fmt.Fprintln(os.Stdout, version.String())
		os.Exit(0)
	}

	return cfg
}

func setupLogger(debug bool) {
	colorizer := lgr.Mapper{
		ErrorFunc:  func(s string) string { return color.New(color.FgHiRed).Sprint(s) },
		WarnFunc:   func(s string) string { return color.New(color.FgHiYellow).Sprint(s) },
		InfoFunc:   func(s string) string { return color.New(color.FgGreen).Sprint(s) },
		DebugFunc:  func(s string) string { return color.New(color.FgWhite).Sprint(s) },
		CallerFunc: func(s string) string { return color.New(color.FgBlue).Sprint(s) },
		TimeFunc:   func(s string) string { return color.New(color.FgCyan).Sprint(s) },
	}
	logOpts := []lgr.Option{lgr.LevelBraces, lgr.Map(colorizer)}
	if debug {
		logOpts = append(logOpts, []lgr.Option{lgr.Debug, lgr.CallerPkg, lgr.CallerFile, lgr.CallerFunc}...)
	}
	lgr.SetupStdLogger(logOpts...)
}
