package sqlite

import (
	"context"
	"database/sql"

	"github.com/agalitsyn/telegram-tasks-bot/internal/model"
)

type UserStorage struct {
	db *sql.DB
}

func NewUserStorage(db *sql.DB) *UserStorage {
	return &UserStorage{db: db}
}

func (s *UserStorage) FetchUserRoleInProject(ctx context.Context, projectID int, user *model.User) error {
	const query = `SELECT up.user_role FROM users u
	JOIN user_projects up ON u.id = up.user_id
	WHERE up.project_id = ? AND u.id = ?`

	var roleStr string
	err := s.db.QueryRowContext(ctx, query, projectID, user.ID).Scan(&roleStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return model.ErrUserNotFound
		}
		return err
	}
	user.Role = model.UserProjectRole(roleStr)
	return nil
}

func (s *UserStorage) CreateUser(ctx context.Context, user *model.User) error {
	const query = `INSERT INTO users (tg_user_id, full_name, is_active) VALUES (?, ?, ?)`
	result, err := s.db.ExecContext(ctx, query, user.TgUserID, user.FullName, user.IsActive)
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

func (s *UserStorage) FetchUserByTgID(ctx context.Context, tgUserID int64) (*model.User, error) {
	const query = `SELECT id, tg_user_id, full_name, is_active FROM users WHERE tg_user_id = ?`
	var user model.User
	err := s.db.QueryRowContext(ctx, query, tgUserID).Scan(
		&user.ID,
		&user.TgUserID,
		&user.FullName,
		&user.IsActive,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (s *UserStorage) UpdateUser(ctx context.Context, user *model.User) error {
	const query = ` UPDATE users SET full_name = ?, is_active = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, user.FullName, user.IsActive, user.ID)
	return err
}

func (s *UserStorage) AddUserToProject(ctx context.Context, projectID int, userID int, role model.UserProjectRole) error {
	const query = `INSERT INTO user_projects (user_id, project_id, user_role) VALUES (?, ?, ?) `
	_, err := s.db.ExecContext(ctx, query, userID, projectID, string(role))
	return err
}

func (s *UserStorage) CountUsersInProject(ctx context.Context, projectID int) (int, error) {
	const query = `SELECT COUNT(*) FROM user_projects WHERE project_id = ?`
	var count int
	err := s.db.QueryRowContext(ctx, query, projectID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}
