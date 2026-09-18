package auth

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrEmailAlreadyRegistered = errors.New("email already registered")
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrUnauthorized           = errors.New("unauthorized")
	ErrUserNotFound           = errors.New("user not found")
)

type User struct {
	ID        uuid.UUID
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PasswordUser struct {
	User
	PasswordHash string
}

type RefreshTokenInput struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Hash      [32]byte
	ExpiresAt time.Time
}

type Session struct {
	AccessToken  string
	ExpiresIn    int
	RefreshToken string
	User         User
}
