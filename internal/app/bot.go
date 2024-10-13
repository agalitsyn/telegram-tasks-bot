package app

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/agalitsyn/telegram-tasks-bot/pkg/version"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type BotConfig struct {
	UpdateTimeout int
}

type Bot struct {
	cfg BotConfig
	*tgbotapi.BotAPI
}

func NewBot(cfg BotConfig, token string, logger tgbotapi.BotLogger) (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	tgbotapi.SetLogger(logger)
	return &Bot{
		cfg:    cfg,
		BotAPI: bot,
	}, nil
}

func (b *Bot) Start(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = b.cfg.UpdateTimeout
	updates := b.GetUpdatesChan(u)
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

				if _, err := b.Send(msg); err != nil {
					log.Printf("ERROR send faiiled: %s", err)
				}
				continue
			}

			if err := b.handleCommand(update); err != nil {
				log.Printf("ERROR send failed: %s", err)
			}

		case <-ctx.Done():
			log.Printf("DEBUG stopped: %s", ctx.Err())
			return
		}
	}
}

type commandHandler func(update tgbotapi.Update) (tgbotapi.MessageConfig, error)

var commandHandlers = map[string]commandHandler{
	"help":   helpCommand,
	"status": statusCommand,
	"start":  startCommand,
}

var errUnknownCommand = errors.New("Незнакомая команда.")

func (b *Bot) handleCommand(update tgbotapi.Update) error {
	command := update.Message.Command()

	handler, exists := commandHandlers[command]
	if !exists {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, errUnknownCommand.Error())
		_, err := b.Send(msg)
		return err
	}

	msg, err := handler(update)
	if err != nil {
		return err
	}

	_, err = b.Send(msg)
	return err
}

func helpCommand(update tgbotapi.Update) (tgbotapi.MessageConfig, error) {
	tpl := `Трекер задач

	Создать проект /start
	Создать задачу /create_task
	Статус /status
	Помощь /help

	---
	Версия: %s`
	text := fmt.Sprintf(tpl, version.String())
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
	return msg, nil
}

func statusCommand(update tgbotapi.Update) (tgbotapi.MessageConfig, error) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Работаю.")
	return msg, nil
}

func startCommand(update tgbotapi.Update) (tgbotapi.MessageConfig, error) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Старт.")
	return msg, nil
}
