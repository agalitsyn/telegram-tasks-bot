package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := ParseFlags()
	setupLogger(cfg.Debug)

	if cfg.Debug {
		log.Printf("DEBUG running with config %v", cfg.String())
	}

	bot, err := tgbotapi.NewBotAPI(cfg.Token.Unmask())
	if err != nil {
		log.Fatalf("could not init bot: %s", err)
	}
	log.Printf("INFO authorized account %s", bot.Self.UserName)

	tgbotapi.SetLogger(log.Default())
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
				log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

				msg := tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)
				msg.ReplyToMessageID = update.Message.MessageID

				if _, err := bot.Send(msg); err != nil {
					log.Printf("ERROR send faiiled: %s", err)
				}
				continue
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
				log.Printf("ERROR send faiiled: %s", err)
			}

		case <-ctx.Done():
			log.Printf("DEBUG stopped: %s", ctx.Err())
			return
		}
	}
}
