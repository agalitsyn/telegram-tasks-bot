package sqlite

import (
	"context"
	"database/sql"

	"github.com/agalitsyn/telegram-tasks-bot/internal/model"
)

type ProjectStorage struct {
	db *sql.DB
}

func NewProjectStorage(db *sql.DB) *ProjectStorage {
	return &ProjectStorage{db: db}
}

func (s *ProjectStorage) CreateProject(ctx context.Context, project *model.Project) error {
	const q = `INSERT INTO projects (tg_chat_id, title, archived) VALUES (?, ?, ?)`
	result, err := s.db.ExecContext(ctx, q, project.TgChatID, project.Title, project.Archived)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	project.ID = int(id)
	return nil
}

func (s *ProjectStorage) GetProjectByID(ctx context.Context, id int) (*model.Project, error) {
	const q = `SELECT id, tg_chat_id, title, archived FROM projects WHERE id = ?`
	var project model.Project
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&project.ID,
		&project.TgChatID,
		&project.Title,
		&project.Archived,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrProjectNotFound
		}
		return nil, err
	}
	return &project, nil
}

func (s *ProjectStorage) FetchProjectByChatID(ctx context.Context, tgChatID int64) (*model.Project, error) {
	const q = `SELECT id, tg_chat_id, title, archived FROM projects WHERE tg_chat_id = ?`
	var project model.Project
	err := s.db.QueryRowContext(ctx, q, tgChatID).Scan(
		&project.ID,
		&project.TgChatID,
		&project.Title,
		&project.Archived,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrProjectNotFound
		}
		return nil, err
	}
	return &project, nil
}

func (s *ProjectStorage) UpdateProject(ctx context.Context, project *model.Project) error {
	const q = `UPDATE projects SET title = ?, archived = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, q, project.Title, project.Archived, project.ID)
	return err
}

func (s *ProjectStorage) DeleteProject(ctx context.Context, id int) error {
	const q = `DELETE FROM projects WHERE id = ?`
	_, err := s.db.ExecContext(ctx, q, id)
	return err
}

// func (s *ProjectStorage) ListProjects(ctx context.Context) ([]model.Project, error) {
// 	query := `
// 		SELECT id, tg_chat_id, title, archived
// 		FROM projects
// 		ORDER BY id
// 	`
// 	rows, err := s.db.QueryContext(ctx, query)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer rows.Close()

// 	var projects []model.Project
// 	for rows.Next() {
// 		var project model.Project
// 		err := rows.Scan(
// 			&project.ID,
// 			&project.TgChatID,
// 			&project.Title,
// 			&project.Archived,
// 		)
// 		if err != nil {
// 			return nil, err
// 		}
// 		projects = append(projects, project)
// 	}

// 	if err = rows.Err(); err != nil {
// 		return nil, err
// 	}

// 	return projects, nil
// }

// // Implement TaskRepository methods

// func (s *ProjectStorage) FilterTasks(ctx context.Context, filter model.TaskFilter) ([]model.Task, error) {
// 	query := `
// 		SELECT id, project_id, title, description, status, deadline, created_by, updated_by, assignee
// 		FROM tasks
// 		WHERE project_id = ?
// 	`
// 	args := []interface{}{filter.ProjectID}

// 	if filter.Status != "" {
// 		query += " AND status = ?"
// 		args = append(args, filter.Status)
// 	}
// 	if filter.CreatedBy != 0 {
// 		query += " AND created_by = ?"
// 		args = append(args, filter.CreatedBy)
// 	}
// 	if filter.Assignee != 0 {
// 		query += " AND assignee = ?"
// 		args = append(args, filter.Assignee)
// 	}
// 	if !filter.Deadline.IsZero() {
// 		query += " AND deadline <= ?"
// 		args = append(args, filter.Deadline)
// 	}

// 	rows, err := s.db.QueryContext(ctx, query, args...)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer rows.Close()

// 	var tasks []model.Task
// 	for rows.Next() {
// 		var task model.Task
// 		var deadlineNull sql.NullTime
// 		err := rows.Scan(
// 			&task.ID,
// 			&task.ProjectID,
// 			&task.Title,
// 			&task.Description,
// 			&task.Status,
// 			&deadlineNull,
// 			&task.CreatedBy,
// 			&task.UpdatedBy,
// 			&task.Assignee,
// 		)
// 		if err != nil {
// 			return nil, err
// 		}
// 		if deadlineNull.Valid {
// 			task.Deadline = deadlineNull.Time
// 		}
// 		tasks = append(tasks, task)
// 	}

// 	if err = rows.Err(); err != nil {
// 		return nil, err
// 	}

// 	return tasks, nil
// }

// func (s *ProjectStorage) CreateTask(ctx context.Context, task *model.Task) error {
// 	query := `
// 		INSERT INTO tasks (project_id, title, description, status, deadline, created_by, updated_by, assignee)
// 		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
// 	`
// 	result, err := s.db.ExecContext(ctx, query,
// 		task.ProjectID, task.Title, task.Description, task.Status, task.Deadline, task.CreatedBy, task.UpdatedBy, task.Assignee)
// 	if err != nil {
// 		return err
// 	}

// 	id, err := result.LastInsertId()
// 	if err != nil {
// 		return err
// 	}

// 	task.ID = int(id)
// 	return nil
// }

// func (s *ProjectStorage) UpdateTask(ctx context.Context, task *model.Task) error {
// 	query := `
// 		UPDATE tasks
// 		SET title = ?, description = ?, status = ?, deadline = ?, updated_by = ?, assignee = ?
// 		WHERE id = ?
// 	`
// 	_, err := s.db.ExecContext(ctx, query,
// 		task.Title, task.Description, task.Status, task.Deadline, task.UpdatedBy, task.Assignee, task.ID)
// 	return err
// }

// func (s *ProjectStorage) RemoveTask(ctx context.Context, id int) error {
// 	query := `DELETE FROM tasks WHERE id = ?`
// 	_, err := s.db.ExecContext(ctx, query, id)
// 	return err
// }
