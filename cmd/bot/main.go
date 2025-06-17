package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/agalitsyn/sqlite"

	"github.com/agalitsyn/telegram-tasks-bot/internal/app"
	sqliteStorage "github.com/agalitsyn/telegram-tasks-bot/internal/storage/sqlite"
	"github.com/agalitsyn/telegram-tasks-bot/migrations"
	"github.com/agalitsyn/telegram-tasks-bot/version"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := ParseFlags()
	if cfg.runPrintVersion {
		fmt.Fprintln(os.Stdout, version.String())
		os.Exit(0)
	}

	setupLogger(cfg.Debug)

	if cfg.Debug {
		log.Printf("DEBUG running with config %v", cfg.String())
	}

	db, err := sqlite.Connect("db.sqlite3")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err = sqlite.MigrateUp(db, migrations.FS); err != nil {
		log.Printf("ERROR could not apply migrations: %s", err)
		return
	}
	if cfg.runMigrate {
		os.Exit(0)
	}

	log.Printf("version: %s", version.String())

	projectStorage := sqliteStorage.NewProjectStorage(db)
	userStorage := sqliteStorage.NewUserStorage(db)
	taskStorage := sqliteStorage.NewTaskStorage(db)

	bot, err := app.NewBot(
		app.BotConfig{
			UpdateTimeout: 60,
			Public:        cfg.Public,
			AllowedTgIDs:  cfg.AllowedTgIDs,
		},
		cfg.Token.Unmask(),
		log.Default(),
		projectStorage,
		userStorage,
		taskStorage,
	)
	if err != nil {
		log.Printf("ERROR could not init bot: %s", err)
		return
	}
	if cfg.Debug {
		bot.SetDebug(true)
	}

	log.Printf("INFO starting with authorized account %s", bot.GetSelf().UserName)
	bot.Start(ctx)
}
