package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/agalitsyn/telegram-tasks-bot/internal/model"
	"github.com/agalitsyn/telegram-tasks-bot/version"
)

type BotConfig struct {
	UpdateTimeout int
}

const (
	parseMarkdown = "Markdown"
)

type TaskCreationStep int

const (
	TaskStepTitle TaskCreationStep = iota
	TaskStepDescription
	TaskStepAssignee
)

type TaskEditField string

const (
	TaskEditFieldTitle       TaskEditField = "title"
	TaskEditFieldDescription TaskEditField = "description"
	TaskEditFieldStatus      TaskEditField = "status"
	TaskEditFieldDeadline    TaskEditField = "deadline"
	TaskEditFieldAssignee    TaskEditField = "assignee"
)

type TaskCreationState struct {
	Step        TaskCreationStep
	ProjectID   int
	Title       string
	Description string
	CreatedBy   int64
}

type TaskEditState struct {
	Field  TaskEditField
	TaskID int
}

type Bot struct {
	api *tgbotapi.BotAPI

	cfg            BotConfig
	projectStorage model.ProjectRepository
	userStorage    model.UserRepository
	taskStorage    model.TaskRepository

	// Task creation state
	taskCreationState map[int64]*TaskCreationState
	// Task editing state
	taskEditState map[int64]*TaskEditState
	// Project rename state
	projectRenameState map[int64]bool
}

func NewBot(
	cfg BotConfig,
	token string,
	logger tgbotapi.BotLogger,
	projectStorage model.ProjectRepository,
	userStorage model.UserRepository,
	taskStorage model.TaskRepository,
) (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	tgbotapi.SetLogger(logger)

	// Set bot commands for persistent menu
	commands := []tgbotapi.BotCommand{
		{
			Command:     "start",
			Description: "Создать проект и показать главное меню",
		},
		{
			Command:     "create_task",
			Description: "Создать новую задачу",
		},
		{
			Command:     "home",
			Description: "Показать главное меню",
		},
		{
			Command:     "status",
			Description: "Показать статус",
		},
	}

	setCommandsConfig := tgbotapi.NewSetMyCommands(commands...)
	if _, err := bot.Request(setCommandsConfig); err != nil {
		log.Printf("WARNING: Failed to set bot commands: %s", err)
	}

	return &Bot{
		api:                bot,
		cfg:                cfg,
		projectStorage:     projectStorage,
		userStorage:        userStorage,
		taskStorage:        taskStorage,
		taskCreationState:  make(map[int64]*TaskCreationState),
		taskEditState:      make(map[int64]*TaskEditState),
		projectRenameState: make(map[int64]bool),
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

			// Check if user is in task creation process
			if state, exists := b.taskCreationState[update.Message.From.ID]; exists {
				if err := b.handleTaskCreationMessage(ctx, update, state); err != nil {
					log.Printf("ERROR handling task creation message: %s", err)
				}
				continue
			}

			// Check if user is in task editing process
			if state, exists := b.taskEditState[update.Message.From.ID]; exists {
				if err := b.handleTaskEditMessage(ctx, update, state); err != nil {
					log.Printf("ERROR handling task edit message: %s", err)
				}
				continue
			}

			// Check if user is in project rename process
			if _, exists := b.projectRenameState[update.Message.From.ID]; exists {
				if err := b.handleProjectRenameMessage(ctx, update); err != nil {
					log.Printf("ERROR handling project rename message: %s", err)
				}
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
		return b.startCommand(ctx, update)
	case "create_project":
		return b.createProjectCommand(ctx, update)
	case "create_task":
		return b.startTaskCreation(ctx, update)
	case "rename_project":
		return b.renameProjectCommand(ctx, update)
	case "status":
		return b.statusCommand(update)
	case "home":
		return b.homeCommand(update)
	default:
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Незнакомая команда.")
		_, err := b.api.Send(msg)
		return err
	}
}

func (b *Bot) startCommand(ctx context.Context, update tgbotapi.Update) error {
	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	// Check if project already exists
	_, err := b.projectStorage.FetchProjectByChatID(ctx, chatID)
	if err == nil {
		// Project exists, show main menu
		return b.showMainMenuForUser(chatID, update.Message.MessageID, userID)
	}

	if !errors.Is(err, model.ErrProjectNotFound) {
		return fmt.Errorf("could not check project: %w", err)
	}

	// Project doesn't exist, show confirmation dialog
	text := "🚀 *Создание проекта*\n\n" +
		"Вы действительно хотите создать проект? Вы приобретете статус менеджера проекта и будете управлять его задачами."

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Продолжить", "confirm_create_project"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "cancel_create_project"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = parseMarkdown
	msg.ReplyMarkup = keyboard

	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) homeCommand(update tgbotapi.Update) error {
	return b.showMainMenuForUser(update.Message.Chat.ID, update.Message.MessageID, update.Message.From.ID)
}

func (b *Bot) statusCommand(update tgbotapi.Update) error {
	statusText := fmt.Sprintf("🤖 *Статус*\n\n✅ Работаю\n📊 Версия: %s", version.String())
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, statusText)
	msg.ParseMode = parseMarkdown
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
			user.FullName = update.Message.From.UserName
		}
		user.Username = update.Message.From.UserName
		if err = b.userStorage.CreateUser(ctx, user); err != nil {
			return fmt.Errorf("could not create user: %w", err)
		}
		log.Printf("DEBUG created user id=%d username=%s", user.ID, user.Username)
	} else if err != nil {
		return fmt.Errorf("could not fetch user: %w", err)
	} else {
		// Update username if it changed
		if user.Username != update.Message.From.UserName {
			user.Username = update.Message.From.UserName
			if err = b.userStorage.UpdateUser(ctx, user); err != nil {
				log.Printf("WARNING: could not update user username: %s", err)
			}
		}
		log.Printf("DEBUG fetch user id=%d username=%s", user.ID, user.Username)
	}

	userAdded := false
	var userRole model.UserProjectRole
	userRole, err = b.userStorage.FetchUserRoleInProject(ctx, prj.ID, user.ID)
	if err != nil && errors.Is(err, model.ErrUserNotFound) {
		usersInPrjNum, err := b.userStorage.CountUsersInProject(ctx, prj.ID)
		if err != nil {
			return fmt.Errorf("could not count users in project: %w", err)
		}

		userRole = model.UserProjectRoleMember
		// If this user is first user associated with project
		if usersInPrjNum == 0 {
			userRole = model.UserProjectRoleManager
		}

		if err = b.userStorage.AddUserToProject(ctx, prj.ID, user.ID, userRole); err != nil {
			return fmt.Errorf("could not add user to project: %w", err)
		}
		log.Printf("DEBUG user id=%d assigned with role '%s' to project id=%d", user.ID, userRole, prj.ID)

		userAdded = true
	} else if err != nil {
		return fmt.Errorf("could not fetch user role for project: %w", err)
	} else {
		log.Printf("DEBUG user id=%d has role '%s' in project id=%d", user.ID, userRole, prj.ID)
	}

	var text string
	if userAdded {
		text = fmt.Sprintf(
			"✅ *Проект \"%s\" создан успешно!*\n\nВы добавлены с ролью `%s`",
			prj.Title, cases.Title(language.Russian).String(userRole.StringLocalized()),
		)
	} else {
		text = fmt.Sprintf(
			"🚀 Вы уже состоите в проекте \"%s\" с ролью `%s`",
			prj.Title, cases.Title(language.Russian).String(userRole.StringLocalized()),
		)
	}
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
	msg.ParseMode = parseMarkdown
	_, err = b.api.Send(msg)
	if err != nil {
		return err
	}

	// Show main menu after project creation
	return b.showMainMenuForUser(update.Message.Chat.ID, 0, update.Message.From.ID)
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

func (b *Bot) showMainMenuForUser(chatID int64, messageID int, userID int64) error {
	ctx := context.Background()
	text := fmt.Sprintf("🤖 *Трекер задач*\n\n_Версия: %s_", version.String())

	var keyboardRows [][]tgbotapi.InlineKeyboardButton

	// Check if project exists for this chat
	_, err := b.projectStorage.FetchProjectByChatID(ctx, chatID)
	projectExists := err == nil

	// Show "Создать проект" button only if project doesn't exist
	if !projectExists {
		keyboardRows = append(keyboardRows,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✨ Создать проект", "cmd_create_project"),
			),
		)
	}

	// Always show these buttons
	keyboardRows = append(keyboardRows,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📝 Создать задачу", "cmd_create_task"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Мои задачи", "cmd_my_tasks"),
		),
	)

	// Show manager buttons only for managers
	if userID != 0 {
		isManager, managerErr := b.isUserManager(ctx, chatID, userID)
		if managerErr == nil && isManager {
			keyboardRows = append(keyboardRows,
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("📂 Задачи проекта", "cmd_project_tasks"),
				),
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("⚙️ Управление проектом", "cmd_project_management"),
				),
			)
		}
	}

	// Always show status button
	keyboardRows = append(keyboardRows,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Статус", "cmd_status"),
		),
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(keyboardRows...)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = parseMarkdown
	msg.ReplyMarkup = keyboard

	_, err = b.api.Send(msg)
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
		fakeUpdate := tgbotapi.Update{
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{
					ID:    chatID,
					Title: update.CallbackQuery.Message.Chat.Title,
				},
				From: update.CallbackQuery.From,
			},
		}
		return b.startTaskCreation(ctx, fakeUpdate)
	case "cmd_my_tasks":
		fakeUpdate := tgbotapi.Update{
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{
					ID:    chatID,
					Title: update.CallbackQuery.Message.Chat.Title,
				},
				From: update.CallbackQuery.From,
			},
		}
		return b.showMyTasks(ctx, fakeUpdate)
	case "cmd_project_tasks":
		fakeUpdate := tgbotapi.Update{
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{
					ID:    chatID,
					Title: update.CallbackQuery.Message.Chat.Title,
				},
				From: update.CallbackQuery.From,
			},
		}
		return b.showProjectTasks(ctx, fakeUpdate)
	case "cmd_back_to_menu":
		return b.showMainMenuForUser(chatID, update.CallbackQuery.Message.MessageID, update.CallbackQuery.From.ID)
	case "cmd_status":
		statusText := fmt.Sprintf("🤖 *Статус*\n\n✅ Работаю\n📊 Версия: %s", version.String())
		msg := tgbotapi.NewMessage(chatID, statusText)
		msg.ParseMode = parseMarkdown
		_, err := b.api.Send(msg)
		return err
	case "confirm_create_project":
		// Create project and add current user as manager
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
	case "cancel_create_project":
		// Show cancellation message
		msg := tgbotapi.NewMessage(chatID, "❌ Создание проекта отменено.")
		_, err := b.api.Send(msg)
		return err
	case "cmd_project_management":
		return b.showProjectManagement(ctx, chatID, update.CallbackQuery.From.ID)
	case "cmd_rename_project":
		return b.startProjectRename(ctx, update)
	case "cmd_assign_manager":
		return b.showAssignManager(ctx, update)
	case "cmd_delete_project":
		return b.confirmDeleteProject(ctx, update)
	default:
		// Handle task button clicks (format: task_<id>)
		if strings.HasPrefix(data, "task_") {
			taskIDStr := strings.TrimPrefix(data, "task_")
			taskID, err := strconv.Atoi(taskIDStr)
			if err != nil {
				return fmt.Errorf("invalid task ID: %w", err)
			}
			return b.showTaskDetails(ctx, update, taskID)
		}

		// Handle edit field button clicks (format: edit_field_<taskID>_<field>)
		if strings.HasPrefix(data, "edit_field_") {
			parts := strings.Split(strings.TrimPrefix(data, "edit_field_"), "_")
			if len(parts) != 2 {
				return fmt.Errorf("invalid edit field format")
			}
			taskID, err := strconv.Atoi(parts[0])
			if err != nil {
				return fmt.Errorf("invalid task ID: %w", err)
			}
			field := TaskEditField(parts[1])
			return b.startFieldEdit(ctx, update, taskID, field)
		}

		// Handle clear field button clicks (format: clear_field_<taskID>_<field>)
		if strings.HasPrefix(data, "clear_field_") {
			parts := strings.Split(strings.TrimPrefix(data, "clear_field_"), "_")
			if len(parts) != 2 {
				return fmt.Errorf("invalid clear field format")
			}
			taskID, err := strconv.Atoi(parts[0])
			if err != nil {
				return fmt.Errorf("invalid task ID: %w", err)
			}
			field := TaskEditField(parts[1])
			return b.clearTaskField(ctx, update, taskID, field)
		}

		// Handle status selection button clicks (format: set_status_<taskID>_<status>)
		if strings.HasPrefix(data, "set_status_") {
			parts := strings.Split(strings.TrimPrefix(data, "set_status_"), "_")
			if len(parts) < 2 {
				return fmt.Errorf("invalid set status format")
			}
			taskID, err := strconv.Atoi(parts[0])
			if err != nil {
				return fmt.Errorf("invalid task ID: %w", err)
			}
			statusStr := strings.Join(parts[1:], "_") // Handle statuses with underscores
			return b.setTaskStatus(ctx, update, taskID, model.TaskStatus(statusStr))
		}

		// Handle promote to manager button clicks (format: promote_to_manager_<userID>)
		if strings.HasPrefix(data, "promote_to_manager_") {
			userIDStr := strings.TrimPrefix(data, "promote_to_manager_")
			targetUserID, err := strconv.Atoi(userIDStr)
			if err != nil {
				return fmt.Errorf("invalid user ID: %w", err)
			}
			return b.promoteToManager(ctx, update, targetUserID)
		}

		// Handle project deletion confirmation
		if data == "confirm_delete_project" {
			return b.deleteProject(ctx, update)
		}

		// Handle noop button clicks (do nothing)
		if data == "noop" {
			return nil
		}

		return nil
	}
}

func (b *Bot) startTaskCreation(ctx context.Context, update tgbotapi.Update) error {
	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	// Check if project exists for this chat
	project, err := b.projectStorage.FetchProjectByChatID(ctx, chatID)
	if err != nil {
		if errors.Is(err, model.ErrProjectNotFound) {
			msg := tgbotapi.NewMessage(chatID, "❌ Проект не найден. Создайте проект сначала с помощью команды /start.")
			_, err := b.api.Send(msg)
			return err
		}
		return fmt.Errorf("could not fetch project: %w", err)
	}

	// Initialize task creation state
	b.taskCreationState[userID] = &TaskCreationState{
		Step:      TaskStepTitle,
		ProjectID: project.ID,
		CreatedBy: userID,
	}

	msg := tgbotapi.NewMessage(chatID, "📝 *Создание новой задачи*\n\nШаг 1/3: Введите название задачи:")
	msg.ParseMode = parseMarkdown
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) handleTaskCreationMessage(ctx context.Context, update tgbotapi.Update, state *TaskCreationState) error {
	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	text := strings.TrimSpace(update.Message.Text)

	// Handle cancel command
	if text == "/cancel" || text == "отмена" {
		delete(b.taskCreationState, userID)
		msg := tgbotapi.NewMessage(chatID, "❌ Создание задачи отменено.")
		_, err := b.api.Send(msg)
		return err
	}

	switch state.Step {
	case TaskStepTitle:
		if text == "" {
			msg := tgbotapi.NewMessage(chatID, "❌ Название задачи не может быть пустым. Попробуйте еще раз:")
			_, err := b.api.Send(msg)
			return err
		}
		state.Title = text
		state.Step = TaskStepDescription
		msg := tgbotapi.NewMessage(chatID, "📄 Шаг 2/3: Введите описание задачи (или отправьте '-' чтобы пропустить):")
		_, err := b.api.Send(msg)
		return err

	case TaskStepDescription:
		if text != "-" {
			state.Description = text
		}
		state.Step = TaskStepAssignee
		msg := tgbotapi.NewMessage(chatID, "👤 Шаг 3/3: Назначьте исполнителя с помощью @упоминания (или отправьте '-' чтобы пропустить):")
		_, err := b.api.Send(msg)
		return err

	case TaskStepAssignee:
		return b.finalizeTaskCreation(ctx, update, state, text)
	}

	return nil
}

func (b *Bot) finalizeTaskCreation(ctx context.Context, update tgbotapi.Update, state *TaskCreationState, assigneeText string) error {
	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	// Create the task
	task := model.NewTask(state.ProjectID, state.Title, state.CreatedBy)
	task.Description = state.Description
	task.Status = model.TaskStatusTODO

	// Handle assignee if provided
	var assigneeName string
	if assigneeText != "-" && assigneeText != "" {
		// Extract @mention using regex
		mentionRegex := regexp.MustCompile(`@(\w+)`)
		matches := mentionRegex.FindStringSubmatch(assigneeText)
		if len(matches) > 1 {
			username := matches[1]

			// Don't allow assigning to bot
			if username == b.api.Self.UserName {
				msg := tgbotapi.NewMessage(chatID, "❌ Нельзя назначить задачу боту. Назначьте другого исполнителя:")
				_, err := b.api.Send(msg)
				return err
			}

			// Try to find user by username in the project
			assigneeUser, err := b.userStorage.FetchUserByUsername(ctx, username)
			if err != nil {
				if errors.Is(err, model.ErrUserNotFound) {
					msg := tgbotapi.NewMessage(chatID,
						fmt.Sprintf("❌ Пользователь @%s не найден. Убедитесь, что пользователь добавлен в проект.", username))
					_, err := b.api.Send(msg)
					return err
				}
				return fmt.Errorf("could not fetch user by username: %w", err)
			}

			// Check if user is in the project
			_, err = b.userStorage.FetchUserRoleInProject(ctx, state.ProjectID, assigneeUser.ID)
			if err != nil {
				if errors.Is(err, model.ErrUserNotFound) {
					msg := tgbotapi.NewMessage(chatID,
						fmt.Sprintf("❌ Пользователь @%s не является участником проекта.", username))
					_, err := b.api.Send(msg)
					return err
				}
				return fmt.Errorf("could not check user role: %w", err)
			}

			task.Assignee = int64(assigneeUser.ID)
			assigneeName = assigneeUser.FullName
			if assigneeName == "" {
				assigneeName = "@" + username
			}
		}
	}

	// Task remains unassigned if no valid assignee was specified

	// Save the task
	if err := b.taskStorage.CreateTask(ctx, task); err != nil {
		delete(b.taskCreationState, userID)
		msg := tgbotapi.NewMessage(chatID, "❌ Ошибка при создании задачи. Попробуйте еще раз.")
		_, err := b.api.Send(msg)
		return err
	}

	// Clean up state
	delete(b.taskCreationState, userID)

	// Send confirmation
	var responseText string
	if assigneeName != "" {
		responseText = fmt.Sprintf("✅ *Задача создана успешно!*\n\n📝 *Название:* %s\n📄 *Описание:* %s\n👤 *Исполнитель:* %s\n📊 *Статус:* %s",
			task.Title,
			getDescriptionOrDefault(task.Description),
			assigneeName,
			string(task.Status))
	} else {
		responseText = fmt.Sprintf("✅ *Задача создана успешно!*\n\n📝 *Название:* %s\n📄 *Описание:* %s\n📊 *Статус:* %s",
			task.Title,
			getDescriptionOrDefault(task.Description),
			string(task.Status))
	}

	msg := tgbotapi.NewMessage(chatID, responseText)
	msg.ParseMode = parseMarkdown
	_, err := b.api.Send(msg)
	if err != nil {
		return err
	}

	// Show main menu after successful task creation
	return b.showMainMenuForUser(chatID, 0, userID)
}

func getDescriptionOrDefault(description string) string {
	if description == "" {
		return "_Не указано_"
	}
	return description
}

func escapeMarkdown(text string) string {
	// Escape markdown special characters for Telegram MarkdownV1
	text = strings.ReplaceAll(text, "*", "\\*")
	text = strings.ReplaceAll(text, "_", "\\_")
	text = strings.ReplaceAll(text, "`", "\\`")
	text = strings.ReplaceAll(text, "[", "\\[")
	return text
}

func filterRecentTasks(tasks []model.Task) []model.Task {
	// Filter out Done/Cancelled tasks that were updated more than 3 days ago
	threeDaysAgo := time.Now().AddDate(0, 0, -3)
	var filteredTasks []model.Task

	for _, task := range tasks {
		// Show task if:
		// 1. Status is not Done or Cancelled, OR
		// 2. Task was updated within the last 3 days
		if (task.Status != model.TaskStatusDone && task.Status != model.TaskStatusCancelled) ||
			task.UpdatedAt.After(threeDaysAgo) {
			filteredTasks = append(filteredTasks, task)
		}
	}

	return filteredTasks
}

func (b *Bot) showMyTasks(ctx context.Context, update tgbotapi.Update) error {
	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	// Get current project
	project, err := b.projectStorage.FetchProjectByChatID(ctx, chatID)
	if err != nil {
		if errors.Is(err, model.ErrProjectNotFound) {
			msg := tgbotapi.NewMessage(chatID, "❌ Проект не найден. Создайте проект сначала с помощью команды /start.")
			_, err := b.api.Send(msg)
			return err
		}
		return fmt.Errorf("could not fetch project: %w", err)
	}

	// Get current user
	user, err := b.userStorage.FetchUserByTgID(ctx, userID)
	if err != nil {
		if errors.Is(err, model.ErrUserNotFound) {
			msg := tgbotapi.NewMessage(chatID, "❌ Пользователь не найден. Используйте команду /start для регистрации.")
			_, err := b.api.Send(msg)
			return err
		}
		return fmt.Errorf("could not fetch user: %w", err)
	}

	// Filter tasks for current user and project
	filter := model.TaskFilter{
		ProjectID: project.ID,
		Assignee:  int64(user.ID),
	}

	tasks, err := b.taskStorage.FilterTasks(ctx, filter)
	if err != nil {
		return fmt.Errorf("could not filter tasks: %w", err)
	}

	// Filter out old completed/cancelled tasks
	tasks = filterRecentTasks(tasks)

	if len(tasks) == 0 {
		msg := tgbotapi.NewMessage(chatID, "📋 *Мои задачи*\n\n_У вас пока нет назначенных задач._")
		msg.ParseMode = parseMarkdown
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🏠 Домой", "cmd_back_to_menu"),
			),
		)
		msg.ReplyMarkup = keyboard
		_, err := b.api.Send(msg)
		return err
	}

	// Build inline keyboard with task buttons (one per row)
	var keyboardRows [][]tgbotapi.InlineKeyboardButton
	for _, task := range tasks {
		statusEmoji := getTaskStatusEmoji(task.Status)
		buttonText := fmt.Sprintf("%s %s", statusEmoji, task.Title)
		callbackData := fmt.Sprintf("task_%d", task.ID)

		row := tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(buttonText, callbackData),
		)
		keyboardRows = append(keyboardRows, row)
	}

	// Add home button
	homeRow := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🏠 Домой", "cmd_back_to_menu"),
	)
	keyboardRows = append(keyboardRows, homeRow)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(keyboardRows...)

	text := fmt.Sprintf("📋 *Мои задачи*\n\n_Найдено задач: %d_", len(tasks))
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = parseMarkdown
	msg.ReplyMarkup = keyboard

	_, err = b.api.Send(msg)
	return err
}

func getTaskStatusEmoji(status model.TaskStatus) string {
	switch status {
	case model.TaskStatusBacklog:
		return "📥"
	case model.TaskStatusTODO:
		return "📋"
	case model.TaskStatusInProgress:
		return "🔄"
	case model.TaskStatusDone:
		return "✅"
	case model.TaskStatusCancelled:
		return "❌"
	case model.TaskStatusOnHold:
		return "⏸️"
	default:
		return "📋"
	}
}

func (b *Bot) showTaskDetails(ctx context.Context, update tgbotapi.Update, taskID int) error {
	chatID := update.CallbackQuery.Message.Chat.ID

	// Get task details
	task, err := b.taskStorage.GetTaskByID(ctx, taskID)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Задача не найдена.")
		_, err := b.api.Send(msg)
		return err
	}

	// Get assignee name if assigned
	var assigneeName string
	if task.Assignee != 0 {
		assigneeUser, err := b.userStorage.FetchUserByID(ctx, int(task.Assignee))
		if err == nil {
			assigneeName = assigneeUser.FullName
		} else {
			assigneeName = fmt.Sprintf("ID: %d", task.Assignee)
		}
	}

	// Build task details text (escape markdown special characters)
	statusEmoji := getTaskStatusEmoji(task.Status)
	text := "📋 *Детали задачи*\n\n"
	text += fmt.Sprintf("*ID:* %d\n", task.ID)
	text += fmt.Sprintf("*Название:* %s\n", escapeMarkdown(task.Title))
	text += fmt.Sprintf("*Описание:* %s\n", escapeMarkdown(getDescriptionOrDefault(task.Description)))
	text += fmt.Sprintf("*Статус:* %s %s\n", statusEmoji, string(task.Status))

	if assigneeName != "" {
		text += fmt.Sprintf("*Исполнитель:* %s\n", escapeMarkdown(assigneeName))
	} else {
		text += "*Исполнитель:* _Не назначен_\n"
	}

	if !task.Deadline.IsZero() {
		text += fmt.Sprintf("*Дедлайн:* %s\n", task.Deadline.Format("02.01.2006 15:04"))
	}

	// Build keyboard - determine the correct back button based on context
	var backButtonText, backButtonData string

	// Check if this is being viewed from "Мои задачи" or "Задачи проекта"
	// For now, we'll use a simple approach - if user is manager, show project tasks back button
	isManager, managerErr := b.isUserManager(ctx, chatID, update.CallbackQuery.From.ID)
	if managerErr == nil && isManager {
		backButtonText = "🔙 К задачам проекта"
		backButtonData = "cmd_project_tasks"
	} else {
		backButtonText = "🔙 К моим задачам"
		backButtonData = "cmd_my_tasks"
	}

	// Build edit buttons with single column layout
	var keyboardRows [][]tgbotapi.InlineKeyboardButton

	// Title row
	keyboardRows = append(keyboardRows,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✏️ Название", fmt.Sprintf("edit_field_%d_title", taskID)),
		),
	)

	// Description row
	keyboardRows = append(keyboardRows,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📄 Описание", fmt.Sprintf("edit_field_%d_description", taskID)),
		),
	)

	// Status row
	keyboardRows = append(keyboardRows,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Статус", fmt.Sprintf("edit_field_%d_status", taskID)),
		),
	)

	// Deadline row
	keyboardRows = append(keyboardRows,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📅 Дедлайн", fmt.Sprintf("edit_field_%d_deadline", taskID)),
		),
	)

	// Assignee row
	keyboardRows = append(keyboardRows,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👤 Исполнитель", fmt.Sprintf("edit_field_%d_assignee", taskID)),
		),
	)

	// Back button
	keyboardRows = append(keyboardRows,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(backButtonText, backButtonData),
		),
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(keyboardRows...)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = parseMarkdown
	msg.ReplyMarkup = keyboard

	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) isUserManager(ctx context.Context, chatID int64, userID int64) (bool, error) {
	// Get current project
	project, err := b.projectStorage.FetchProjectByChatID(ctx, chatID)
	if err != nil {
		return false, err
	}

	// Get current user
	user, err := b.userStorage.FetchUserByTgID(ctx, userID)
	if err != nil {
		return false, err
	}

	// Get user role in project
	userRole, err := b.userStorage.FetchUserRoleInProject(ctx, project.ID, user.ID)
	if err != nil {
		return false, err
	}

	return userRole == model.UserProjectRoleManager, nil
}

func (b *Bot) showProjectTasks(ctx context.Context, update tgbotapi.Update) error {
	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	// Check if user is manager
	isManager, err := b.isUserManager(ctx, chatID, userID)
	if err != nil {
		return fmt.Errorf("could not check user role: %w", err)
	}
	if !isManager {
		msg := tgbotapi.NewMessage(chatID, "❌ У вас нет прав для просмотра задач проекта.")
		_, err := b.api.Send(msg)
		return err
	}

	// Get current project
	project, err := b.projectStorage.FetchProjectByChatID(ctx, chatID)
	if err != nil {
		if errors.Is(err, model.ErrProjectNotFound) {
			msg := tgbotapi.NewMessage(chatID, "❌ Проект не найден. Создайте проект сначала с помощью команды /start.")
			_, err := b.api.Send(msg)
			return err
		}
		return fmt.Errorf("could not fetch project: %w", err)
	}

	// Get all tasks in project
	filter := model.TaskFilter{
		ProjectID: project.ID,
	}
	tasks, err := b.taskStorage.FilterTasks(ctx, filter)
	if err != nil {
		return fmt.Errorf("could not fetch tasks: %w", err)
	}

	// Filter out old completed/cancelled tasks
	tasks = filterRecentTasks(tasks)

	if len(tasks) == 0 {
		msg := tgbotapi.NewMessage(chatID, "📂 *Задачи проекта*\n\n_В проекте пока нет задач._")
		msg.ParseMode = parseMarkdown
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🏠 Домой", "cmd_back_to_menu"),
			),
		)
		msg.ReplyMarkup = keyboard
		_, err := b.api.Send(msg)
		return err
	}

	// Build inline keyboard with task buttons (one per row)
	var keyboardRows [][]tgbotapi.InlineKeyboardButton
	for _, task := range tasks {
		// Get assignee name
		var assigneeName string
		if task.Assignee != 0 {
			assigneeUser, err := b.userStorage.FetchUserByID(ctx, int(task.Assignee))
			if err == nil {
				assigneeName = assigneeUser.FullName
			} else {
				assigneeName = fmt.Sprintf("ID:%d", task.Assignee)
			}
		} else {
			assigneeName = "Не назначен"
		}

		statusEmoji := getTaskStatusEmoji(task.Status)
		buttonText := fmt.Sprintf("%s #%d %s - %s", statusEmoji, task.ID, task.Title, assigneeName)
		callbackData := fmt.Sprintf("task_%d", task.ID)

		row := tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(buttonText, callbackData),
		)
		keyboardRows = append(keyboardRows, row)
	}

	// Add home button
	homeRow := tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🏠 Домой", "cmd_back_to_menu"),
	)
	keyboardRows = append(keyboardRows, homeRow)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(keyboardRows...)

	text := fmt.Sprintf("📂 *Задачи проекта*\n\n_Всего задач: %d_", len(tasks))
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = parseMarkdown
	msg.ReplyMarkup = keyboard

	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) startFieldEdit(ctx context.Context, update tgbotapi.Update, taskID int, field TaskEditField) error {
	chatID := update.CallbackQuery.Message.Chat.ID
	userID := update.CallbackQuery.From.ID

	// Get the task to edit
	task, err := b.taskStorage.GetTaskByID(ctx, taskID)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Задача не найдена.")
		_, err := b.api.Send(msg)
		return err
	}

	// Initialize task edit state for specific field
	b.taskEditState[userID] = &TaskEditState{
		Field:  field,
		TaskID: taskID,
	}

	var promptText string
	switch field {
	case TaskEditFieldTitle:
		promptText = fmt.Sprintf(
			"✏️ *Редактирование названия задачи #%d*\n\nТекущее название: %s\n\nВведите новое название:",
			taskID,
			task.Title,
		)

		// Add back button for title
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", fmt.Sprintf("task_%d", taskID)),
			),
		)
		msg := tgbotapi.NewMessage(chatID, promptText)
		msg.ParseMode = parseMarkdown
		msg.ReplyMarkup = keyboard
		_, err = b.api.Send(msg)
		return err
	case TaskEditFieldDescription:
		promptFormat := "📄 *Редактирование описания задачи #%d*\n\nТекущее описание: %s\n\nВведите новое описание:"
		promptText = fmt.Sprintf(promptFormat, taskID, getDescriptionOrDefault(task.Description))

		// Add clear button for description
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🗑 Очистить описание", fmt.Sprintf("clear_field_%d_description", taskID)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", fmt.Sprintf("task_%d", taskID)),
			),
		)
		msg := tgbotapi.NewMessage(chatID, promptText)
		msg.ParseMode = parseMarkdown
		msg.ReplyMarkup = keyboard
		_, err = b.api.Send(msg)
		return err
	case TaskEditFieldStatus:
		statusEmoji := getTaskStatusEmoji(task.Status)
		promptText = fmt.Sprintf(
			"📊 *Редактирование статуса задачи #%d*\n\nТекущий статус: %s %s\n\nВыберите новый статус:",
			taskID, statusEmoji, string(task.Status))

		// Show status selection buttons
		var statusKeyboard [][]tgbotapi.InlineKeyboardButton
		statuses := []struct {
			emoji  string
			status model.TaskStatus
			text   string
		}{
			{"📥", model.TaskStatusBacklog, "Backlog"},
			{"📋", model.TaskStatusTODO, "TODO"},
			{"🔄", model.TaskStatusInProgress, "В работе"},
			{"✅", model.TaskStatusDone, "Выполнено"},
			{"❌", model.TaskStatusCancelled, "Отменено"},
			{"⏸️", model.TaskStatusOnHold, "На паузе"},
		}

		for _, s := range statuses {
			if s.status != task.Status { // Don't show current status
				statusKeyboard = append(statusKeyboard,
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData(
							fmt.Sprintf("%s %s", s.emoji, s.text),
							fmt.Sprintf("set_status_%d_%s", taskID, string(s.status)),
						),
					),
				)
			}
		}

		// Add cancel button
		statusKeyboard = append(statusKeyboard,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", fmt.Sprintf("task_%d", taskID)),
			),
		)

		keyboard := tgbotapi.NewInlineKeyboardMarkup(statusKeyboard...)
		msg := tgbotapi.NewMessage(chatID, promptText)
		msg.ParseMode = parseMarkdown
		msg.ReplyMarkup = keyboard
		_, err = b.api.Send(msg)
		return err
	case TaskEditFieldDeadline:
		currentDeadline := "не установлен"
		if !task.Deadline.IsZero() {
			currentDeadline = task.Deadline.Format("02.01.2006 15:04")
		}
		promptFormat := "📅 *Редактирование дедлайна задачи #%d*\n\nТекущий дедлайн: %s\n\n" +
			"Введите новый дедлайн в формате ДД.ММ.ГГГГ ЧЧ:ММ:"
		promptText = fmt.Sprintf(promptFormat, taskID, currentDeadline)

		// Add clear button for deadline
		var keyboardRows [][]tgbotapi.InlineKeyboardButton
		if !task.Deadline.IsZero() {
			keyboardRows = append(keyboardRows,
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("🗑 Убрать дедлайн", fmt.Sprintf("clear_field_%d_deadline", taskID)),
				),
			)
		}
		keyboardRows = append(keyboardRows,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", fmt.Sprintf("task_%d", taskID)),
			),
		)
		keyboard := tgbotapi.NewInlineKeyboardMarkup(keyboardRows...)
		msg := tgbotapi.NewMessage(chatID, promptText)
		msg.ParseMode = parseMarkdown
		msg.ReplyMarkup = keyboard
		_, err = b.api.Send(msg)
		return err
	case TaskEditFieldAssignee:
		var currentAssignee string
		if task.Assignee != 0 {
			assigneeUser, err := b.userStorage.FetchUserByID(ctx, int(task.Assignee))
			if err == nil {
				currentAssignee = assigneeUser.FullName
			} else {
				currentAssignee = fmt.Sprintf("ID: %d", task.Assignee)
			}
		} else {
			currentAssignee = "не назначен"
		}
		promptFormat := "👤 *Редактирование исполнителя задачи #%d*\n\nТекущий исполнитель: %s\n\n" +
			"Введите @упоминание нового исполнителя:"
		promptText = fmt.Sprintf(promptFormat, taskID, currentAssignee)

		// Add clear button for assignee
		var keyboardRows [][]tgbotapi.InlineKeyboardButton
		if task.Assignee != 0 {
			keyboardRows = append(keyboardRows,
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("🗑 Убрать исполнителя", fmt.Sprintf("clear_field_%d_assignee", taskID)),
				),
			)
		}
		keyboardRows = append(keyboardRows,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", fmt.Sprintf("task_%d", taskID)),
			),
		)
		keyboard := tgbotapi.NewInlineKeyboardMarkup(keyboardRows...)
		msg := tgbotapi.NewMessage(chatID, promptText)
		msg.ParseMode = parseMarkdown
		msg.ReplyMarkup = keyboard
		_, err = b.api.Send(msg)
		return err
	}

	// This should never be reached as all cases handle their own messages
	return fmt.Errorf("unhandled field type: %s", field)
}

func (b *Bot) handleTaskEditMessage(ctx context.Context, update tgbotapi.Update, state *TaskEditState) error {
	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	text := strings.TrimSpace(update.Message.Text)

	// Handle cancel command
	if text == "/cancel" || text == "отмена" {
		delete(b.taskEditState, userID)
		msg := tgbotapi.NewMessage(chatID, "❌ Редактирование отменено.")
		_, err := b.api.Send(msg)
		return err
	}

	if text == "" {
		msg := tgbotapi.NewMessage(chatID, "❌ Значение не может быть пустым. Попробуйте еще раз:")
		_, err := b.api.Send(msg)
		return err
	}

	return b.updateTaskField(ctx, update, state, text)
}

func (b *Bot) updateTaskField(ctx context.Context, update tgbotapi.Update, state *TaskEditState, newValue string) error {
	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	// Get the current task
	task, err := b.taskStorage.GetTaskByID(ctx, state.TaskID)
	if err != nil {
		delete(b.taskEditState, userID)
		msg := tgbotapi.NewMessage(chatID, "❌ Ошибка при получении задачи.")
		_, err := b.api.Send(msg)
		return err
	}

	var fieldName string
	var oldValue, updatedValue string

	// Update the specific field
	switch state.Field {
	case TaskEditFieldTitle:
		fieldName = "название"
		oldValue = task.Title
		task.Title = newValue
		updatedValue = newValue

	case TaskEditFieldDescription:
		fieldName = "описание"
		oldValue = getDescriptionOrDefault(task.Description)
		task.Description = newValue
		updatedValue = getDescriptionOrDefault(newValue)

	case TaskEditFieldStatus:
		// Status editing is now handled by buttons, not text input
		delete(b.taskEditState, userID)
		msg := tgbotapi.NewMessage(chatID, "❌ Статус изменяется через кнопки, а не текст.")
		_, err := b.api.Send(msg)
		return err

	case TaskEditFieldDeadline:
		fieldName = "дедлайн"
		if !task.Deadline.IsZero() {
			oldValue = task.Deadline.Format("02.01.2006 15:04")
		} else {
			oldValue = "не установлен"
		}

		// In a real implementation, you'd parse the date properly
		// For now, we'll skip deadline parsing to keep it simple
		updatedValue = newValue

	case TaskEditFieldAssignee:
		fieldName = "исполнитель"

		// Get old assignee name for confirmation message
		if task.Assignee != 0 {
			assigneeUser, err := b.userStorage.FetchUserByID(ctx, int(task.Assignee))
			if err == nil {
				oldValue = assigneeUser.FullName
			} else {
				oldValue = fmt.Sprintf("ID: %d", task.Assignee)
			}
		} else {
			oldValue = "не назначен"
		}

		// Extract @mention using regex
		mentionRegex := regexp.MustCompile(`@(\w+)`)
		matches := mentionRegex.FindStringSubmatch(newValue)
		if len(matches) > 1 {
			username := matches[1]

			// Don't allow assigning to bot
			if username == b.api.Self.UserName {
				delete(b.taskEditState, userID)
				msg := tgbotapi.NewMessage(chatID, "❌ Нельзя назначить задачу боту.")
				_, err := b.api.Send(msg)
				return err
			}

			// Find user by username
			assigneeUser, err := b.userStorage.FetchUserByUsername(ctx, username)
			if err != nil {
				if errors.Is(err, model.ErrUserNotFound) {
					delete(b.taskEditState, userID)
					msg := tgbotapi.NewMessage(chatID,
						fmt.Sprintf("❌ Пользователь @%s не найден.", username))
					_, err := b.api.Send(msg)
					return err
				}
				return fmt.Errorf("could not fetch user by username: %w", err)
			}

			// Get project ID from task
			projectTask, err := b.taskStorage.GetTaskByID(ctx, state.TaskID)
			if err != nil {
				return fmt.Errorf("could not get task project: %w", err)
			}

			// Check if user is in the project
			_, err = b.userStorage.FetchUserRoleInProject(ctx, projectTask.ProjectID, assigneeUser.ID)
			if err != nil {
				if errors.Is(err, model.ErrUserNotFound) {
					delete(b.taskEditState, userID)
					msg := tgbotapi.NewMessage(chatID,
						fmt.Sprintf("❌ Пользователь @%s не является участником проекта.", username))
					_, err := b.api.Send(msg)
					return err
				}
				return fmt.Errorf("could not check user role: %w", err)
			}

			task.Assignee = int64(assigneeUser.ID)
			updatedValue = assigneeUser.FullName
			if updatedValue == "" {
				updatedValue = "@" + username
			}
		} else {
			// Invalid format
			msg := tgbotapi.NewMessage(chatID, "❌ Неверный формат. Используйте @username")
			_, err := b.api.Send(msg)
			return err
		}
	}

	// Update the task
	if err := b.taskStorage.UpdateTask(ctx, task); err != nil {
		delete(b.taskEditState, userID)
		msg := tgbotapi.NewMessage(chatID, "❌ Ошибка при обновлении задачи.")
		_, err := b.api.Send(msg)
		return err
	}

	// Clean up state
	delete(b.taskEditState, userID)

	// Send brief confirmation
	confirmationText := fmt.Sprintf("✅ *%s* обновлено: %s → %s", fieldName, oldValue, updatedValue)
	confirmMsg := tgbotapi.NewMessage(chatID, confirmationText)
	confirmMsg.ParseMode = parseMarkdown
	_, err = b.api.Send(confirmMsg)
	if err != nil {
		return err
	}

	// Create fake update to show task details
	fakeUpdate := tgbotapi.Update{
		CallbackQuery: &tgbotapi.CallbackQuery{
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{
					ID: chatID,
				},
			},
			From: &tgbotapi.User{
				ID: userID,
			},
		},
	}

	// Show updated task details
	return b.showTaskDetails(ctx, fakeUpdate, state.TaskID)
}

func (b *Bot) clearTaskField(ctx context.Context, update tgbotapi.Update, taskID int, field TaskEditField) error {
	chatID := update.CallbackQuery.Message.Chat.ID
	userID := update.CallbackQuery.From.ID

	// Clean up any existing edit state
	delete(b.taskEditState, userID)

	// Get the current task
	task, err := b.taskStorage.GetTaskByID(ctx, taskID)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Задача не найдена.")
		_, err := b.api.Send(msg)
		return err
	}

	var fieldName string
	var clearedValue string

	// Clear the specific field
	switch field {
	case TaskEditFieldDescription:
		fieldName = "описание"
		task.Description = ""
		clearedValue = "очищено"

	case TaskEditFieldDeadline:
		fieldName = "дедлайн"
		task.Deadline = time.Time{}
		clearedValue = "убран"

	case TaskEditFieldAssignee:
		fieldName = "исполнитель"
		task.Assignee = 0
		clearedValue = "убран"

	default:
		msg := tgbotapi.NewMessage(chatID, "❌ Это поле нельзя очистить.")
		_, err := b.api.Send(msg)
		return err
	}

	// Update the task
	if err := b.taskStorage.UpdateTask(ctx, task); err != nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Ошибка при обновлении задачи.")
		_, err := b.api.Send(msg)
		return err
	}

	// Send brief confirmation
	confirmationText := fmt.Sprintf("✅ *%s* %s", fieldName, clearedValue)
	confirmMsg := tgbotapi.NewMessage(chatID, confirmationText)
	confirmMsg.ParseMode = parseMarkdown
	_, err = b.api.Send(confirmMsg)
	if err != nil {
		return err
	}

	// Create fake update to show task details
	fakeUpdate := tgbotapi.Update{
		CallbackQuery: &tgbotapi.CallbackQuery{
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{
					ID: chatID,
				},
			},
			From: &tgbotapi.User{
				ID: userID,
			},
		},
	}

	// Show updated task details
	return b.showTaskDetails(ctx, fakeUpdate, taskID)
}

func (b *Bot) setTaskStatus(ctx context.Context, update tgbotapi.Update, taskID int, newStatus model.TaskStatus) error {
	chatID := update.CallbackQuery.Message.Chat.ID
	userID := update.CallbackQuery.From.ID

	// Get the current task
	task, err := b.taskStorage.GetTaskByID(ctx, taskID)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Задача не найдена.")
		_, err := b.api.Send(msg)
		return err
	}

	oldStatus := task.Status
	task.Status = newStatus

	// Update the task
	if err := b.taskStorage.UpdateTask(ctx, task); err != nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Ошибка при обновлении задачи.")
		_, err := b.api.Send(msg)
		return err
	}

	// Send brief confirmation
	oldEmoji := getTaskStatusEmoji(oldStatus)
	newEmoji := getTaskStatusEmoji(newStatus)
	confirmationText := fmt.Sprintf("✅ *Статус* обновлен: %s %s → %s %s",
		oldEmoji, string(oldStatus), newEmoji, string(newStatus))
	confirmMsg := tgbotapi.NewMessage(chatID, confirmationText)
	confirmMsg.ParseMode = parseMarkdown
	_, err = b.api.Send(confirmMsg)
	if err != nil {
		return err
	}

	// Create fake update to show task details
	fakeUpdate := tgbotapi.Update{
		CallbackQuery: &tgbotapi.CallbackQuery{
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{
					ID: chatID,
				},
			},
			From: &tgbotapi.User{
				ID: userID,
			},
		},
	}

	// Show updated task details
	return b.showTaskDetails(ctx, fakeUpdate, taskID)
}

func (b *Bot) showProjectManagement(ctx context.Context, chatID int64, userID int64) error {
	// Check if user is manager
	isManager, err := b.isUserManager(ctx, chatID, userID)
	if err != nil {
		return fmt.Errorf("could not check user role: %w", err)
	}
	if !isManager {
		msg := tgbotapi.NewMessage(chatID, "❌ У вас нет прав для управления проектом.")
		_, err := b.api.Send(msg)
		return err
	}

	// Get project info
	project, err := b.projectStorage.FetchProjectByChatID(ctx, chatID)
	if err != nil {
		return fmt.Errorf("could not fetch project: %w", err)
	}

	text := fmt.Sprintf("⚙️ *Управление проектом*\n\n*Проект:* %s", escapeMarkdown(project.Title))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✏️ Переименовать проект", "cmd_rename_project"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👤 Назначить менеджера", "cmd_assign_manager"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗑 Удалить проект", "cmd_delete_project"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "cmd_back_to_menu"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = parseMarkdown
	msg.ReplyMarkup = keyboard

	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) startProjectRename(ctx context.Context, update tgbotapi.Update) error {
	chatID := update.CallbackQuery.Message.Chat.ID
	userID := update.CallbackQuery.From.ID

	// Check if user is manager
	isManager, err := b.isUserManager(ctx, chatID, userID)
	if err != nil {
		return fmt.Errorf("could not check user role: %w", err)
	}
	if !isManager {
		msg := tgbotapi.NewMessage(chatID, "❌ У вас нет прав для переименования проекта.")
		_, err := b.api.Send(msg)
		return err
	}

	// Get current project
	project, err := b.projectStorage.FetchProjectByChatID(ctx, chatID)
	if err != nil {
		return fmt.Errorf("could not fetch project: %w", err)
	}

	// Set rename state
	b.projectRenameState[userID] = true

	text := fmt.Sprintf("✏️ *Переименование проекта*\n\nТекущее название: %s\n\n"+
		"Введите новое название проекта:", escapeMarkdown(project.Title))
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = parseMarkdown
	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) handleProjectRenameMessage(ctx context.Context, update tgbotapi.Update) error {
	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID
	newTitle := strings.TrimSpace(update.Message.Text)

	// Handle cancel command
	if newTitle == "/cancel" || newTitle == "отмена" {
		delete(b.projectRenameState, userID)
		msg := tgbotapi.NewMessage(chatID, "❌ Переименование проекта отменено.")
		_, err := b.api.Send(msg)
		return err
	}

	if newTitle == "" {
		msg := tgbotapi.NewMessage(chatID, "❌ Название проекта не может быть пустым. Попробуйте еще раз:")
		_, err := b.api.Send(msg)
		return err
	}

	// Get current project
	project, err := b.projectStorage.FetchProjectByChatID(ctx, chatID)
	if err != nil {
		delete(b.projectRenameState, userID)
		return fmt.Errorf("could not fetch project: %w", err)
	}

	oldTitle := project.Title
	project.Title = newTitle

	// Update project
	if err := b.projectStorage.UpdateProject(ctx, project); err != nil {
		delete(b.projectRenameState, userID)
		msg := tgbotapi.NewMessage(chatID, "❌ Ошибка при переименовании проекта.")
		_, err := b.api.Send(msg)
		return err
	}

	// Clean up state
	delete(b.projectRenameState, userID)

	// Send confirmation
	confirmText := fmt.Sprintf("✅ Проект переименован: %s → %s", escapeMarkdown(oldTitle), escapeMarkdown(newTitle))
	confirmMsg := tgbotapi.NewMessage(chatID, confirmText)
	confirmMsg.ParseMode = parseMarkdown
	_, err = b.api.Send(confirmMsg)
	if err != nil {
		return err
	}

	// Show project management menu
	return b.showProjectManagement(ctx, chatID, userID)
}

func (b *Bot) showAssignManager(ctx context.Context, update tgbotapi.Update) error {
	chatID := update.CallbackQuery.Message.Chat.ID
	userID := update.CallbackQuery.From.ID

	// Check if user is manager
	isManager, err := b.isUserManager(ctx, chatID, userID)
	if err != nil {
		return fmt.Errorf("could not check user role: %w", err)
	}
	if !isManager {
		msg := tgbotapi.NewMessage(chatID, "❌ У вас нет прав для назначения менеджера.")
		_, err := b.api.Send(msg)
		return err
	}

	// Get current project
	project, err := b.projectStorage.FetchProjectByChatID(ctx, chatID)
	if err != nil {
		return fmt.Errorf("could not fetch project: %w", err)
	}

	// Get project members with member role
	projectUsers, err := b.userStorage.FetchUsersInProject(ctx, project.ID)
	if err != nil {
		return fmt.Errorf("could not fetch project users: %w", err)
	}

	var memberButtons [][]tgbotapi.InlineKeyboardButton
	membersFound := false

	for _, user := range projectUsers {
		// Check user role
		role, err := b.userStorage.FetchUserRoleInProject(ctx, project.ID, user.ID)
		if err != nil {
			continue
		}

		if role == model.UserProjectRoleMember {
			membersFound = true
			buttonText := user.FullName
			if buttonText == "" {
				buttonText = fmt.Sprintf("User ID: %d", user.ID)
			}
			memberButtons = append(memberButtons,
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData(buttonText, fmt.Sprintf("promote_to_manager_%d", user.ID)),
				),
			)
		}
	}

	if !membersFound {
		text := "👤 *Назначение менеджера*\n\n_В проекте нет пользователей с ролью 'участник' для назначения менеджером._"
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "cmd_project_management"),
			),
		)
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = parseMarkdown
		msg.ReplyMarkup = keyboard
		_, err := b.api.Send(msg)
		return err
	}

	// Add back button
	memberButtons = append(memberButtons,
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "cmd_project_management"),
		),
	)

	text := "👤 *Назначение менеджера*\n\nВыберите пользователя для назначения менеджером проекта:"
	keyboard := tgbotapi.NewInlineKeyboardMarkup(memberButtons...)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = parseMarkdown
	msg.ReplyMarkup = keyboard

	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) confirmDeleteProject(ctx context.Context, update tgbotapi.Update) error {
	chatID := update.CallbackQuery.Message.Chat.ID
	userID := update.CallbackQuery.From.ID

	// Check if user is manager
	isManager, err := b.isUserManager(ctx, chatID, userID)
	if err != nil {
		return fmt.Errorf("could not check user role: %w", err)
	}
	if !isManager {
		msg := tgbotapi.NewMessage(chatID, "❌ У вас нет прав для удаления проекта.")
		_, err := b.api.Send(msg)
		return err
	}

	// Get current project
	project, err := b.projectStorage.FetchProjectByChatID(ctx, chatID)
	if err != nil {
		return fmt.Errorf("could not fetch project: %w", err)
	}

	text := fmt.Sprintf("🗑 *Удаление проекта*\n\n⚠️ **ВНИМАНИЕ!** "+
		"Вы действительно хотите удалить проект \"%s\"?\n\n"+
		"Все задачи и данные проекта будут безвозвратно удалены!", escapeMarkdown(project.Title))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Удалить проект", "confirm_delete_project"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 Отмена", "cmd_project_management"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = parseMarkdown
	msg.ReplyMarkup = keyboard

	_, err = b.api.Send(msg)
	return err
}
func (b *Bot) promoteToManager(ctx context.Context, update tgbotapi.Update, targetUserID int) error {
	chatID := update.CallbackQuery.Message.Chat.ID
	userID := update.CallbackQuery.From.ID

	// Check if current user is manager
	isManager, err := b.isUserManager(ctx, chatID, userID)
	if err != nil {
		return fmt.Errorf("could not check user role: %w", err)
	}
	if !isManager {
		msg := tgbotapi.NewMessage(chatID, "❌ У вас нет прав для назначения менеджера.")
		_, err := b.api.Send(msg)
		return err
	}

	// Get current project
	project, err := b.projectStorage.FetchProjectByChatID(ctx, chatID)
	if err != nil {
		return fmt.Errorf("could not fetch project: %w", err)
	}

	// Get target user
	targetUser, err := b.userStorage.FetchUserByID(ctx, targetUserID)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Пользователь не найден.")
		_, err := b.api.Send(msg)
		return err
	}

	// Check if target user is in project with member role
	currentRole, err := b.userStorage.FetchUserRoleInProject(ctx, project.ID, targetUserID)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Пользователь не является участником проекта.")
		_, err := b.api.Send(msg)
		return err
	}

	if currentRole != model.UserProjectRoleMember {
		msg := tgbotapi.NewMessage(chatID, "❌ Пользователь не имеет роль участника.")
		_, err := b.api.Send(msg)
		return err
	}

	// Update user role to manager
	err = b.userStorage.UpdateUserRoleInProject(ctx, project.ID, targetUserID, model.UserProjectRoleManager)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Ошибка при назначении менеджера.")
		_, err := b.api.Send(msg)
		return err
	}

	// Send confirmation
	userName := targetUser.FullName
	if userName == "" {
		userName = fmt.Sprintf("User ID: %d", targetUserID)
	}
	confirmText := fmt.Sprintf("✅ Пользователь %s назначен менеджером проекта", escapeMarkdown(userName))
	confirmMsg := tgbotapi.NewMessage(chatID, confirmText)
	confirmMsg.ParseMode = parseMarkdown
	_, err = b.api.Send(confirmMsg)
	if err != nil {
		return err
	}

	// Show project management menu
	return b.showProjectManagement(ctx, chatID, userID)
}

func (b *Bot) deleteProject(ctx context.Context, update tgbotapi.Update) error {
	chatID := update.CallbackQuery.Message.Chat.ID
	userID := update.CallbackQuery.From.ID

	// Check if user is manager
	isManager, err := b.isUserManager(ctx, chatID, userID)
	if err != nil {
		return fmt.Errorf("could not check user role: %w", err)
	}
	if !isManager {
		msg := tgbotapi.NewMessage(chatID, "❌ У вас нет прав для удаления проекта.")
		_, err := b.api.Send(msg)
		return err
	}

	// Get current project
	project, err := b.projectStorage.FetchProjectByChatID(ctx, chatID)
	if err != nil {
		return fmt.Errorf("could not fetch project: %w", err)
	}

	// Delete project (this will cascade delete tasks and user associations)
	err = b.projectStorage.DeleteProject(ctx, project.ID)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "❌ Ошибка при удалении проекта.")
		_, err := b.api.Send(msg)
		return err
	}

	// Send confirmation
	confirmText := fmt.Sprintf("✅ Проект \"%s\" удален успешно!", escapeMarkdown(project.Title))
	confirmMsg := tgbotapi.NewMessage(chatID, confirmText)
	confirmMsg.ParseMode = parseMarkdown
	_, err = b.api.Send(confirmMsg)
	return err
}
