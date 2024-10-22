package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/agalitsyn/telegram-tasks-bot/internal/model"
	"github.com/agalitsyn/telegram-tasks-bot/pkg/version"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type BotConfig struct {
	UpdateTimeout      int
	InlineQueryEnabled bool
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
			if update.InlineQuery != nil && b.cfg.InlineQueryEnabled {
				if err := b.handleInlineQuery(update); err != nil {
					log.Printf("ERROR handling inline query: %s", err)
				}
				continue
			}

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
		prj = model.NewProject(update.Message.Chat.Title, tgChatID)
		if err = b.projectStorage.CreateProject(ctx, prj); err != nil {
			return fmt.Errorf("could not create project: %w", err)
		}
		log.Printf("DEBUG created project id=%d", prj.ID)
	} else if err != nil {
		return fmt.Errorf("could not fetch project: %w", err)
	} else {
		log.Printf("DEBUG fetch project id=%d", prj.ID)
	}

	user, err := b.userStorage.FetchUserByTgID(ctx, update.Message.From.ID)
	if err != nil && errors.Is(err, model.ErrUserNotFound) {
		user = model.NewUser(update.Message.From.ID)
		if update.Message.From.LastName != "" && update.Message.From.FirstName != "" {
			user.FullName = fmt.Sprintf("%s %s", update.Message.From.LastName, update.Message.From.FirstName)
		} else if update.Message.From.UserName != "" {
			// TODO: update.Message.From.UserName always set?
			user.FullName = update.Message.From.UserName
		}
		if err = b.userStorage.CreateUser(ctx, user); err != nil {
			return fmt.Errorf("could not create user: %w", err)
		}
		log.Printf("DEBUG created user id=%d", user.ID)
	} else if err != nil {
		return fmt.Errorf("could not fetch user: %w", err)
	} else {
		log.Printf("DEBUG fetch user id=%d", user.ID)
	}

	userAdded := false
	err = b.userStorage.FetchUserRoleInProject(ctx, prj.ID, user)
	if err != nil && errors.Is(err, model.ErrUserNotFound) {
		usersInPrjNum, err := b.userStorage.CountUsersInProject(ctx, prj.ID)
		if err != nil {
			return fmt.Errorf("could not count users in project: %w", err)
		}

		user.Role = model.UserProjectRoleMember
		// If this user is first user associated with project
		if usersInPrjNum == 0 {
			user.Role = model.UserProjectRoleManager
		}

		if err = b.userStorage.AddUserToProject(ctx, prj.ID, user.ID, user.Role); err != nil {
			return fmt.Errorf("could not add user to project: %w", err)
		}
		log.Printf("DEBUG user id=%d assigned with role '%s' to project id=%d", user.ID, user.Role, prj.ID)

		userAdded = true
	} else if err != nil {
		return fmt.Errorf("could not fetch user role for project: %w", err)
	} else {
		log.Printf("DEBUG user id=%d has role '%s' in project id=%d", user.ID, user.Role, prj.ID)
	}

	var text string
	if userAdded {
		text = fmt.Sprintf("Ваш пользователь %s добавлен в проект \"%s\" с ролью %s",
			user.FullName, prj.Title, strings.Title(user.Role.StringLocalized()))
	} else {
		text = fmt.Sprintf("Ваш пользователь %s уже находится в проекте \"%s\" с ролью %s",
			user.FullName, prj.Title, strings.Title(user.Role.StringLocalized()))
	}
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
	_, err = b.Send(msg)
	return err
}

func (b *Bot) handleInlineQuery(update tgbotapi.Update) error {
	// TODO: this is example handler

	query := update.InlineQuery.Query
	log.Printf("DEBUG inline query: %s", query)

	result := tgbotapi.NewInlineQueryResultArticle(update.InlineQuery.ID, "Inline title", "Message content")
	result.Description = "Inline Description"

	inlineConf := tgbotapi.InlineConfig{
		InlineQueryID: update.InlineQuery.ID,
		Results:       []interface{}{result},
		CacheTime:     300,
	}

	_, err := b.Request(inlineConf)
	return err
}
