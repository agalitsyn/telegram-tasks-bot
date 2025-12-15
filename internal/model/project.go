package model

import (
	"context"
	"errors"
)

type HideCompletedMode string

const (
	HideCompletedModeShowAll   HideCompletedMode = "show_all"
	HideCompletedModeHideAll   HideCompletedMode = "hide_all"
	HideCompletedModeShowLast3 HideCompletedMode = "show_last_3"
)

type Project struct {
	ID                int
	TgChatID          int64
	Title             string
	Description       string
	Archived          bool
	HideCompletedMode HideCompletedMode
}

func NewProject(title string, tgChatID int64) *Project {
	return &Project{
		Title:             title,
		TgChatID:          tgChatID,
		HideCompletedMode: HideCompletedModeShowLast3,
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
