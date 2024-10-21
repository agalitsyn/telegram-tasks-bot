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
	Role     UserProjectRole
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
	CreateUser(ctx context.Context, user *User) error
	AddUserToProject(ctx context.Context, projectID int, userID int, role UserProjectRole) error
	FetchUserRoleInProject(ctx context.Context, projectID int, user *User) error
	CountUsersInProject(ctx context.Context, projectID int) (int, error)
}
