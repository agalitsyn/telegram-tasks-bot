package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/agalitsyn/telegram-tasks-bot/pkg/slogtools"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := ParseFlags()
	slogtools.SetupGlobalLogger(cfg.Log.Level, os.Stdout)

	if cfg.Debug {
		slog.Debug("running with config")
		fmt.Fprintln(os.Stdout, cfg.String())
	}

	bot, err := tgbotapi.NewBotAPI(cfg.Token.Unmask())
	if err != nil {
		slogtools.Fatal("could not init bot", "err", err)
	}
	slog.Info("authorized", "account", bot.Self.UserName)

	// tgbotapi.SetLogger(&BotDebugLogger{})
	if cfg.Debug {
		bot.Debug = true
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)
	for {
		select {
		case update := <-updates:
			if update.Message == nil { // ignore any non-Message updates
				continue
			}

			if !update.Message.IsCommand() {
				// echo
				slog.Debug("", update.Message.From.UserName, update.Message.Text)

				msg := tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)
				msg.ReplyToMessageID = update.Message.MessageID

				if _, err := bot.Send(msg); err != nil {
					slog.Error("send failed", "err", err)
				}
			}

			// Create a new MessageConfig. We don't have text yet,
			// so we leave it empty.
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

			// Extract the command from the Message.
			switch update.Message.Command() {
			case "help":
				msg.Text = "I understand /sayhi and /status."
			case "sayhi":
				msg.Text = "Hi :)"
			case "status":
				msg.Text = "I'm ok."
			default:
				msg.Text = "I don't know that command"
			}

			if _, err := bot.Send(msg); err != nil {
				slog.Error("send failed", "err", err)
			}

		case <-ctx.Done():
			slog.Debug("stopped", "err", ctx.Err())
			return
		}
	}
}

type BotDebugLogger struct{}

func (l BotDebugLogger) Printf(msg string, args ...interface{}) {
	slog.Debug(fmt.Sprintf(msg, args...))
}

func (l BotDebugLogger) Println(v ...interface{}) {
	slog.Debug("bot:", v...)
}
