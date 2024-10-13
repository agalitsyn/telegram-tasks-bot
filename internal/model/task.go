package model

import "time"

type Task struct {
	ID          int
	ProjectID   int
	Title       string
	Description string
	Status      TaskStatus
	Deadline    time.Time
	CreatedBy   int64
	Assignee    int64
}

type TaskStatus string

const (
	TaskStatusTODO       TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusDone       TaskStatus = "done"
	TaskStatusCancelled  TaskStatus = "cancelled"
	TaskStatusOnHold     TaskStatus = "on_hold"
)
