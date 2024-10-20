package app

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/agalitsyn/telegram-tasks-bot/internal/model"
	"github.com/agalitsyn/telegram-tasks-bot/pkg/version"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type BotConfig struct {
	UpdateTimeout int
}

type Bot struct {
	*tgbotapi.BotAPI

	cfg            BotConfig
	projectStorage model.ProjectRepository
	userStorage    model.UserRepository
}

func NewBot(
	cfg BotConfig,
	token string,
	logger tgbotapi.BotLogger,
	projectStorage model.ProjectRepository,
	userStorage model.UserRepository,
) (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	tgbotapi.SetLogger(logger)
	return &Bot{
		cfg:            cfg,
		projectStorage: projectStorage,
		userStorage:    userStorage,
		BotAPI:         bot,
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

			if err := b.handleCommand(ctx, update); err != nil {
				log.Printf("ERROR send failed: %s", err)
			}

		case <-ctx.Done():
			log.Printf("DEBUG stopped: %s", ctx.Err())
			return
		}
	}
}

func (b *Bot) handleCommand(ctx context.Context, update tgbotapi.Update) error {
	command := update.Message.Command()
	switch command {
	case "start":
		return b.startCommand(ctx, update)
	case "status":
		return b.statusCommand(update)
	case "help":
		return b.helpCommand(update)
	default:
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Незнакомая команда.")
		_, err := b.Send(msg)
		return err
	}
}

func (b *Bot) helpCommand(update tgbotapi.Update) error {
	tpl := `Трекер задач

	Создать проект /start
	Создать задачу /create_task
	Статус /status
	Помощь /help

	---
	Версия: %s`

	text := fmt.Sprintf(tpl, version.String())
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
	_, err := b.Send(msg)
	return err
}

func (b *Bot) statusCommand(update tgbotapi.Update) error {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Работаю.")
	_, err := b.Send(msg)
	return err
}

func (b *Bot) startCommand(ctx context.Context, update tgbotapi.Update) error {
	tgChatID := update.Message.Chat.ID

	prj, err := b.projectStorage.FetchProjectByChatID(ctx, update.Message.Chat.ID)
	if err != nil && errors.Is(err, model.ErrProjectNotFound) {
		// TODO: do we need title?
		prj = model.NewProject("test", tgChatID)
		if err = b.projectStorage.CreateProject(ctx, prj); err != nil {
			return fmt.Errorf("could not create project: %w", err)
		}

		user := model.NewUser(update.Message.From.ID)
		user.Role = model.UserProjectRoleManager
		if update.Message.From.LastName != "" && update.Message.From.FirstName != "" {
			user.FullName = fmt.Sprintf("%s %s", update.Message.From.LastName, update.Message.From.FirstName)
		} else if update.Message.From.UserName != "" {
			user.FullName = update.Message.From.UserName
		}

		if err = b.userStorage.CreateUser(ctx, user); err != nil {
			return fmt.Errorf("could not create user: %w", err)
		}
	}
	if err != nil {
		return fmt.Errorf("could not fetch project: %w", err)
	}

	user, err := b.userStorage.FetchUserInProject(ctx, prj.ID, update.Message.From.ID)
	if err != nil {
		return fmt.Errorf("could not fetch user: %w", err)
	}

	// TODO: remove
	fmt.Printf("(prj): %#v\n", prj)
	fmt.Printf("(user): %#v\n", user)

	msg := tgbotapi.NewMessage(
		update.Message.Chat.ID,
		fmt.Sprintf("проект id=%d юзер id=%d", prj.ID, user.ID),
	)
	_, err = b.Send(msg)
	return err
}
