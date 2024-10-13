package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/agalitsyn/telegram-tasks-bot/pkg/flagtools"
	"github.com/agalitsyn/telegram-tasks-bot/pkg/secret"

	"github.com/fatih/color"
	"github.com/go-pkgz/lgr"
)

const EnvPrefix = "TG_TASKS_BOT"

type Config struct {
	Debug bool
	Token secret.String

	runPrintVersion bool
	runMigrate bool
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
	flag.BoolVar(&cfg.runPrintVersion, "version", false, "Show version.")
	flag.BoolVar(&cfg.runMigrate ,  "migrate", false, "Migrate.")

	flagtools.Prefix = EnvPrefix
	flagtools.Parse()
	flag.Parse()

	cfg.Token = secret.NewString(*token)
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
