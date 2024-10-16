package model

import (
	"context"
	"time"
)

type Task struct {
	ID          int
	ProjectID   int
	Title       string
	Description string
	Status      TaskStatus
	Deadline    time.Time
	CreatedBy   int64
	UpdatedBy   int64
	Assignee    int64
}

func NewTask(projectID int, title string, createdBy int64) *Task {
	return &Task{
		ProjectID: projectID,
		Title:     title,
		CreatedBy: createdBy,
		UpdatedBy: createdBy,
	}
}

type TaskStatus string

const (
	TaskStatusBacklog    TaskStatus = "backlog"
	TaskStatusTODO       TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusDone       TaskStatus = "done"
	TaskStatusCancelled  TaskStatus = "cancelled"
	TaskStatusOnHold     TaskStatus = "on_hold"
)

type TaskFilter struct {
	ProjectID int
	Status    TaskStatus
	CreatedBy int64
	Assignee  int64
	Deadline  time.Time
}

type TaskRepository interface {
	FilterTasks(ctx context.Context, filter TaskFilter) ([]Task, error)
	CreateTask(ctx context.Context, task *Task) error
	UpdateTask(ctx context.Context, task *Task) error
	RemoveTask(ctx context.Context, id int) error
}
