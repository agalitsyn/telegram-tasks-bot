package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/agalitsyn/telegram-tasks-bot/internal/model"
)

type UserStorage struct {
	db *sql.DB
}

func NewUserStorage(db *sql.DB) *UserStorage {
	return &UserStorage{db: db}
}

func (s *UserStorage) FetchUserInProject(ctx context.Context, projectID int, tgUserID int64) (*model.User, error) {
	query := `
		SELECT u.id, u.tg_user_id, u.full_name, up.user_role, u.is_active
		FROM users u
		JOIN user_projects up ON u.id = up.user_id
		WHERE up.project_id = ? AND u.tg_user_id = ?
	`
	var user model.User
	var roleStr string

	err := s.db.QueryRowContext(ctx, query, projectID, tgUserID).Scan(
		&user.ID,
		&user.TgUserID,
		&user.FullName,
		&roleStr,
		&user.IsActive,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrUserNotFound
		}
		return nil, err
	}

	user.Role = model.UserProjectRole(roleStr)

	return &user, nil
}

func (s *UserStorage) CreateUser(ctx context.Context, user *model.User) error {
	query := `
		INSERT INTO users (tg_user_id, full_name, role, is_active)
		VALUES (?, ?, ?, ?)
	`
	result, err := s.db.ExecContext(ctx, query, user.TgUserID, user.FullName, string(user.Role), user.IsActive)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	user.ID = int(id)
	return nil
}

// Additional helper methods

func (s *UserStorage) GetUserByTgUserID(ctx context.Context, tgUserID int64) (*model.User, error) {
	query := `
		SELECT id, tg_user_id, full_name, role, is_active
		FROM users
		WHERE tg_user_id = ?
	`
	var user model.User
	var roleStr string

	err := s.db.QueryRowContext(ctx, query, tgUserID).Scan(
		&user.ID,
		&user.TgUserID,
		&user.FullName,
		&roleStr,
		&user.IsActive,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	user.Role = model.UserProjectRole(roleStr)

	return &user, nil
}

func (s *UserStorage) UpdateUser(ctx context.Context, user *model.User) error {
	query := `
		UPDATE users
		SET full_name = ?, role = ?, is_active = ?
		WHERE id = ?
	`
	_, err := s.db.ExecContext(ctx, query, user.FullName, string(user.Role), user.IsActive, user.ID)
	return err
}

func (s *UserStorage) AddUserToProject(ctx context.Context, userID int, projectID int, role model.UserProjectRole) error {
	query := `
		INSERT INTO user_projects (user_id, project_id, user_role)
		VALUES (?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, query, userID, projectID, string(role))
	return err
}
