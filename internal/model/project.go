package model

import "context"

type Project struct {
	ID       int
	ChatID   int64
	Title    string
	IsActive bool
}

type ProjectStorage interface {
	FetchProjectByChatID(ctx context.Context, chatID int64) (*Project, error)
	CreateProject(ctx context.Context, project *Project) error
	UpdateProject(ctx context.Context, project *Project) error
	DeleteProject(ctx context.Context, id int) error
}
