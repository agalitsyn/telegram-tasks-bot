package model

import (
	"context"
	"errors"
	"fmt"
)

type User struct {
	ID       int
	TgUserID int64
	FullName string
	Username string
	IsActive bool
}

func NewUser(tgUserID int64) *User {
	return &User{
		TgUserID: tgUserID,
		IsActive: true,
	}
}

type UserProjectRole string

func (r UserProjectRole) StringLocalized() string {
	switch r {
	case UserProjectRoleManager:
		return "менеджер"
	case UserProjectRoleMember:
		return "участик"
	default:
		panic(fmt.Sprintf("missing localization for %s", r))
	}
}

const (
	UserProjectRoleManager UserProjectRole = "manager"
	UserProjectRoleMember  UserProjectRole = "member"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

type UserRepository interface {
	FetchUserByTgID(ctx context.Context, tgUserID int64) (*User, error)
	FetchUserByID(ctx context.Context, userID int) (*User, error)
	FetchUserByUsername(ctx context.Context, username string) (*User, error)
	CreateUser(ctx context.Context, user *User) error
	UpdateUser(ctx context.Context, user *User) error
	AddUserToProject(ctx context.Context, projectID int, userID int, role UserProjectRole) error
	FetchUserRoleInProject(ctx context.Context, projectID int, userID int) (UserProjectRole, error)
	UpdateUserRoleInProject(ctx context.Context, projectID int, userID int, role UserProjectRole) error
	CountUsersInProject(ctx context.Context, projectID int) (int, error)
	FetchUsersInProject(ctx context.Context, projectID int) ([]User, error)
}
