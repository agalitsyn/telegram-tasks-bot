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

func (s *UserStorage) FetchUserRoleInProject(ctx context.Context, projectID int, userID int) (model.UserProjectRole, error) {
	const query = `SELECT up.user_role FROM user_projects up WHERE up.project_id = ? AND up.user_id = ?`

	var roleStr string
	err := s.db.QueryRowContext(ctx, query, projectID, userID).Scan(&roleStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", model.ErrUserNotFound
		}
		return "", err
	}
	return model.UserProjectRole(roleStr), nil
}

func (s *UserStorage) CreateUser(ctx context.Context, user *model.User) error {
	const query = `INSERT INTO users (tg_user_id, full_name, username, is_active) VALUES (?, ?, ?, ?)`
	result, err := s.db.ExecContext(ctx, query, user.TgUserID, user.FullName, user.Username, user.IsActive)
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
	const query = `SELECT id, tg_user_id, full_name, username, is_active FROM users WHERE tg_user_id = ?`
	var user model.User
	var username sql.NullString
	err := s.db.QueryRowContext(ctx, query, tgUserID).Scan(
		&user.ID,
		&user.TgUserID,
		&user.FullName,
		&username,
		&user.IsActive,
	)
	if err == nil && username.Valid {
		user.Username = username.String
	}
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (s *UserStorage) FetchUserByID(ctx context.Context, userID int) (*model.User, error) {
	const query = `SELECT id, tg_user_id, full_name, username, is_active FROM users WHERE id = ?`
	var user model.User
	var username sql.NullString
	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&user.ID,
		&user.TgUserID,
		&user.FullName,
		&username,
		&user.IsActive,
	)
	if err == nil && username.Valid {
		user.Username = username.String
	}
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (s *UserStorage) UpdateUser(ctx context.Context, user *model.User) error {
	const query = ` UPDATE users SET full_name = ?, username = ?, is_active = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, user.FullName, user.Username, user.IsActive, user.ID)
	return err
}

func (s *UserStorage) FetchUserByUsername(ctx context.Context, username string) (*model.User, error) {
	const query = `SELECT id, tg_user_id, full_name, username, is_active FROM users WHERE username = ?`
	var user model.User
	var usernameNull sql.NullString
	err := s.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID,
		&user.TgUserID,
		&user.FullName,
		&usernameNull,
		&user.IsActive,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrUserNotFound
		}
		return nil, err
	}
	if usernameNull.Valid {
		user.Username = usernameNull.String
	}
	return &user, nil
}

func (s *UserStorage) AddUserToProject(ctx context.Context, projectID int, userID int, role model.UserProjectRole) error {
	const query = `INSERT INTO user_projects (user_id, project_id, user_role) VALUES (?, ?, ?) `
	_, err := s.db.ExecContext(ctx, query, userID, projectID, string(role))
	return err
}

func (s *UserStorage) UpdateUserRoleInProject(ctx context.Context, projectID int, userID int, role model.UserProjectRole) error {
	const query = `UPDATE user_projects SET user_role = ? WHERE project_id = ? AND user_id = ?`
	_, err := s.db.ExecContext(ctx, query, string(role), projectID, userID)
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

func (s *UserStorage) FetchUsersInProject(ctx context.Context, projectID int) ([]model.User, error) {
	const query = `
		SELECT u.id, u.tg_user_id, u.full_name, u.username, u.is_active
		FROM users u
		JOIN user_projects up ON u.id = up.user_id
		WHERE up.project_id = ?
		ORDER BY u.full_name ASC
	`

	rows, err := s.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var user model.User
		var username sql.NullString

		err := rows.Scan(
			&user.ID,
			&user.TgUserID,
			&user.FullName,
			&username,
			&user.IsActive,
		)
		if err == nil && username.Valid {
			user.Username = username.String
		}
		if err != nil {
			return nil, err
		}

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}
