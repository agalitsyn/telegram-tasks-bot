package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/agalitsyn/flagutils"
	"github.com/agalitsyn/secret"

	"github.com/fatih/color"
	"github.com/go-pkgz/lgr"
)

const EnvPrefix = "TG_TASKS_BOT"

type Config struct {
	Debug        bool
	Token        secret.String
	Public       bool
	AllowedTgIDs []int64

	runPrintVersion bool
	runMigrate      bool
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
	public := flag.Bool("public", true, "Allow all users to use the bot.")
	allowedTgIds := flag.String("allowed-tg-ids", "", "Comma-separated list of allowed Telegram user IDs (only used when public=false).")
	flag.BoolVar(&cfg.runPrintVersion, "version", false, "Show version.")
	flag.BoolVar(&cfg.runMigrate, "migrate", false, "Migrate.")

	flagutils.Prefix = EnvPrefix
	flagutils.Parse()
	flag.Parse()

	cfg.Token = secret.NewString(*token)
	cfg.Public = *public
	cfg.AllowedTgIDs = parseAllowedTgIDs(*allowedTgIds)
	return cfg
}

func parseAllowedTgIDs(allowedTgIdsStr string) []int64 {
	if allowedTgIdsStr == "" {
		return nil
	}

	parts := strings.Split(allowedTgIdsStr, ",")
	allowedTgIDs := make([]int64, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			fmt.Printf("WARNING: invalid Telegram user ID '%s', skipping\n", part)
			continue
		}

		allowedTgIDs = append(allowedTgIDs, id)
	}

	return allowedTgIDs
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
