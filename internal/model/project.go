package model

import (
	"context"
	"errors"
)

type Project struct {
	ID          int
	TgChatID    int64
	Title       string
	Description string
	Archived    bool
}

func NewProject(title string, tgChatID int64) *Project {
	return &Project{
		Title:    title,
		TgChatID: tgChatID,
	}
}

var (
	ErrProjectNotFound = errors.New("project not found")
)

type ProjectRepository interface {
	FetchProjectByChatID(ctx context.Context, tgChatID int64) (*Project, error)
	CreateProject(ctx context.Context, project *Project) error
	UpdateProject(ctx context.Context, project *Project) error
	DeleteProject(ctx context.Context, id int) error
}
