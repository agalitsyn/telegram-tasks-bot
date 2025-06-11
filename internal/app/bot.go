package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/agalitsyn/telegram-tasks-bot/internal/model"
	"github.com/agalitsyn/telegram-tasks-bot/version"
)

type BotConfig struct {
	UpdateTimeout int
}

type Bot struct {
	api *tgbotapi.BotAPI

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
		api:            bot,
		cfg:            cfg,
		projectStorage: projectStorage,
		userStorage:    userStorage,
	}, nil
}

func (b *Bot) Start(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = b.cfg.UpdateTimeout
	updates := b.api.GetUpdatesChan(u)
	for {
		select {
		case update := <-updates:
			if update.CallbackQuery != nil {
				if err := b.handleCallbackQuery(ctx, update); err != nil {
					log.Printf("ERROR handling callback query: %s", err)
				}
				continue
			}

			if update.Message == nil { // ignore any non-Message updates
				continue
			}

			if !update.Message.IsCommand() {
				command, ok := parseCommand(update.Message.Text, b.api.Self.UserName)
				if ok {
					// Create a new update with the parsed command
					cmdUpdate := update
					cmdUpdate.Message.Text = "/" + command
					cmdUpdate.Message.Entities = []tgbotapi.MessageEntity{
						{
							Type:   "bot_command",
							Offset: 0,
							Length: len(command) + 1,
						},
					}
					if err := b.handleCommand(ctx, cmdUpdate); err != nil {
						log.Printf("ERROR handling command: %s", err)
					}

					continue
				}
			}

			if err := b.handleCommand(ctx, update); err != nil {
				log.Printf("ERROR handling command: %s", err)
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
		return b.helpCommand(update)
	case "create_project":
		return b.createProjectCommand(ctx, update)
	case "rename_project":
		return b.renameProjectCommand(ctx, update)
	case "status":
		return b.statusCommand(update)
	case "help":
		return b.helpCommand(update)
	default:
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Незнакомая команда.")
		_, err := b.api.Send(msg)
		return err
	}
}

func (b *Bot) helpCommand(update tgbotapi.Update) error {
	return b.showMainMenu(update.Message.Chat.ID, update.Message.MessageID)
}

func (b *Bot) statusCommand(update tgbotapi.Update) error {
	statusText := fmt.Sprintf("🤖 *Статус бота*\n\n✅ Работаю\n📊 Версия: %s", version.String())
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, statusText)
	msg.ParseMode = "Markdown"
	_, err := b.api.Send(msg)
	return err
}

func (b *Bot) createProjectCommand(ctx context.Context, update tgbotapi.Update) error {
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
		text = fmt.Sprintf(
			"✨ Вы добавлены в проект \"%s\" с ролью `%s`",
			prj.Title, cases.Title(language.Russian).String(user.Role.StringLocalized()),
		)
	} else {
		text = fmt.Sprintf(
			"🚀 Вы уже состоите в проекте \"%s\" с ролью `%s`",
			prj.Title, cases.Title(language.Russian).String(user.Role.StringLocalized()),
		)
	}
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) renameProjectCommand(ctx context.Context, update tgbotapi.Update) error {
	return nil
}

func (b *Bot) SetDebug(debug bool) {
	b.api.Debug = debug
}

func (b *Bot) GetSelf() tgbotapi.User {
	return b.api.Self
}

func parseCommand(text string, botUsername string) (string, bool) {
	prefix := "@" + botUsername + " /"
	if strings.HasPrefix(text, prefix) {
		return strings.TrimPrefix(text, prefix), true
	}
	return "", false
}

func (b *Bot) showMainMenu(chatID int64, messageID int) error {
	text := fmt.Sprintf("🤖 *Трекер задач*\n\n_Версия: %s_", version.String())

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✨ Создать проект", "cmd_create_project"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📝 Создать задачу", "cmd_create_task"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Статус", "cmd_status"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard

	_, err := b.api.Send(msg)
	return err
}

func (b *Bot) handleCallbackQuery(ctx context.Context, update tgbotapi.Update) error {
	callback := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
	if _, err := b.api.Request(callback); err != nil {
		log.Printf("ERROR answering callback query: %s", err)
	}

	data := update.CallbackQuery.Data
	chatID := update.CallbackQuery.Message.Chat.ID

	switch data {
	case "cmd_create_project":
		// Create a fake update to call createProjectCommand
		fakeUpdate := tgbotapi.Update{
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{
					ID:    chatID,
					Title: update.CallbackQuery.Message.Chat.Title,
				},
				From: update.CallbackQuery.From,
			},
		}
		return b.createProjectCommand(ctx, fakeUpdate)
	case "cmd_create_task":
		msg := tgbotapi.NewMessage(chatID, "Функция создания задач пока не реализована.")
		_, err := b.api.Send(msg)
		return err
	case "cmd_status":
		statusText := fmt.Sprintf("🤖 *Статус бота*\n\n✅ Работаю\n📊 Версия: %s", version.String())
		msg := tgbotapi.NewMessage(chatID, statusText)
		msg.ParseMode = "Markdown"
		_, err := b.api.Send(msg)
		return err
	default:
		return nil
	}
}
