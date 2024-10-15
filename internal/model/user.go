package model

import "context"

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
		Role:     UserProjectRoleMember,
		IsActive: true,
	}
}

type UserProjectRole string

const (
	UserProjectRoleManager UserProjectRole = "manager"
	UserProjectRoleMember  UserProjectRole = "member"
)

type UserStorage interface {
	FetchUserInProject(ctx context.Context, projectID int, userID int) (*User, error)
	CreateUser(ctx context.Context, user *User) error
}
