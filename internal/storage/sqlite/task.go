package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/agalitsyn/telegram-tasks-bot/internal/model"
)

type TaskStorage struct {
	db *sql.DB
}

func NewTaskStorage(db *sql.DB) *TaskStorage {
	return &TaskStorage{db: db}
}

func (s *TaskStorage) CreateTask(ctx context.Context, task *model.Task) error {
	query := `
		INSERT INTO tasks (project_id, title, description, status, deadline, created_by, updated_by, assignee, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`

	var assignee sql.NullInt64
	if task.Assignee != 0 {
		assignee.Int64 = task.Assignee
		assignee.Valid = true
	}

	var deadline sql.NullTime
	if !task.Deadline.IsZero() {
		deadline.Time = task.Deadline
		deadline.Valid = true
	}

	result, err := s.db.ExecContext(ctx, query,
		task.ProjectID,
		task.Title,
		task.Description,
		string(task.Status),
		deadline,
		task.CreatedBy,
		task.UpdatedBy,
		assignee,
	)
	if err != nil {
		return fmt.Errorf("could not create task: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("could not get last insert id: %w", err)
	}

	task.ID = int(id)
	return nil
}

func (s *TaskStorage) UpdateTask(ctx context.Context, task *model.Task) error {
	query := `
		UPDATE tasks 
		SET title = ?, description = ?, status = ?, deadline = ?, updated_by = ?, assignee = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	var assignee sql.NullInt64
	if task.Assignee != 0 {
		assignee.Int64 = task.Assignee
		assignee.Valid = true
	}

	var deadline sql.NullTime
	if !task.Deadline.IsZero() {
		deadline.Time = task.Deadline
		deadline.Valid = true
	}

	_, err := s.db.ExecContext(ctx, query,
		task.Title,
		task.Description,
		string(task.Status),
		deadline,
		task.UpdatedBy,
		assignee,
		task.ID,
	)
	if err != nil {
		return fmt.Errorf("could not update task: %w", err)
	}

	return nil
}

func (s *TaskStorage) RemoveTask(ctx context.Context, id int) error {
	query := `DELETE FROM tasks WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("could not remove task: %w", err)
	}
	return nil
}

func (s *TaskStorage) FilterTasks(ctx context.Context, filter model.TaskFilter) ([]model.Task, error) {
	query := `
		SELECT id, project_id, title, description, status, deadline, created_by, updated_by, assignee, created_at, updated_at
		FROM tasks
		WHERE 1=1
	`
	args := []interface{}{}

	if filter.ProjectID != 0 {
		query += " AND project_id = ?"
		args = append(args, filter.ProjectID)
	}

	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, string(filter.Status))
	}

	if filter.CreatedBy != 0 {
		query += " AND created_by = ?"
		args = append(args, filter.CreatedBy)
	}

	if filter.Assignee != 0 {
		query += " AND assignee = ?"
		args = append(args, filter.Assignee)
	}

	if !filter.Deadline.IsZero() {
		query += " AND deadline <= ?"
		args = append(args, filter.Deadline)
	}

	query += " ORDER BY id ASC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("could not filter tasks: %w", err)
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		var task model.Task
		var assignee sql.NullInt64
		var deadline sql.NullTime

		err := rows.Scan(
			&task.ID,
			&task.ProjectID,
			&task.Title,
			&task.Description,
			&task.Status,
			&deadline,
			&task.CreatedBy,
			&task.UpdatedBy,
			&assignee,
			&task.CreatedAt,
			&task.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("could not scan task: %w", err)
		}

		if assignee.Valid {
			task.Assignee = assignee.Int64
		}

		if deadline.Valid {
			task.Deadline = deadline.Time
		}

		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate tasks: %w", err)
	}

	return tasks, nil
}

func (s *TaskStorage) GetTaskByID(ctx context.Context, id int) (*model.Task, error) {
	query := `
		SELECT id, project_id, title, description, status, deadline, created_by, updated_by, assignee, created_at, updated_at
		FROM tasks
		WHERE id = ?
	`

	var task model.Task
	var assignee sql.NullInt64
	var deadline sql.NullTime

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&task.ID,
		&task.ProjectID,
		&task.Title,
		&task.Description,
		&task.Status,
		&deadline,
		&task.CreatedBy,
		&task.UpdatedBy,
		&assignee,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("could not get task: %w", err)
	}

	if assignee.Valid {
		task.Assignee = assignee.Int64
	}

	if deadline.Valid {
		task.Deadline = deadline.Time
	}

	return &task, nil
}
