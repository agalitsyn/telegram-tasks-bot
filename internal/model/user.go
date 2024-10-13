package model

import "context"

type User struct {
	ID       int
	TgUserID int64
	FullName string
	Role     UserProjectRole
	IsActive bool
}

type UserProjectRole string

const (
	UserProjectRoleManager UserProjectRole = "manager"
	UserProjectRoleMember  UserProjectRole = "member"
)

type UserStorage interface {
	FetchUserInProject(ctx context.Context, projectID int, userID int) (*User, error)
}
