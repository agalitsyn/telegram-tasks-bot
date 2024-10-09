package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/agalitsyn/telegram-tasks-bot/pkg/flagtools"
	"github.com/agalitsyn/telegram-tasks-bot/pkg/secret"
	"github.com/agalitsyn/telegram-tasks-bot/pkg/slogtools"
	"github.com/agalitsyn/telegram-tasks-bot/pkg/version"
)

const EnvPrefix = "TG_TASKS_BOT"

type Config struct {
	Debug bool

	Log struct {
		Level slog.Level
	}

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

	printVersion := flag.Bool("version", false, "Show version.")
	logLevel := flag.String("log-level", "info", "Log level (debug | info | warn | error).")
	token := flag.String("token", "", "Telegram bot token.")

	flagtools.Prefix = EnvPrefix
	flagtools.Parse()
	flag.Parse()

	slogLevel := slogtools.ParseLogLevel(*logLevel)
	cfg.Log.Level = slogLevel
	if slogLevel == slog.LevelDebug {
		cfg.Debug = true
	}

	cfg.Token = secret.NewString(*token)

	if *printVersion {
		fmt.Fprintln(os.Stdout, version.String())
		os.Exit(0)
	}

	return cfg
}
