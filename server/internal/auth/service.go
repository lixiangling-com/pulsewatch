package auth

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	store  Store
	tokens *TokenManager
	now    func() time.Time
}

func NewService(store Store, tokens *TokenManager) *Service {
	return &Service{store: store, tokens: tokens, now: time.Now}
}

func (s *Service) Register(ctx context.Context, email, password string) (Session, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return Session{}, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return Session{}, err
	}
	user := PasswordUser{User: User{ID: uuid.New(), Email: email}, PasswordHash: hash}
	rawRefresh, refresh, err := s.newRefreshToken(user.ID)
	if err != nil {
		return Session{}, err
	}
	created, err := s.store.Register(ctx, user, refresh)
	if err != nil {
		return Session{}, err
	}
	return s.sessionFor(created, rawRefresh)
}

func (s *Service) Login(ctx context.Context, email, password string) (Session, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return Session{}, ErrInvalidCredentials
	}
	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil || !CheckPassword(user.PasswordHash, password) {
		return Session{}, ErrInvalidCredentials
	}
	rawRefresh, refresh, err := s.newRefreshToken(user.ID)
	if err != nil {
		return Session{}, err
	}
	if err := s.store.CreateRefreshToken(ctx, refresh); err != nil {
		return Session{}, err
	}
	return s.sessionFor(user.User, rawRefresh)
}

func (s *Service) Refresh(ctx context.Context, raw string) (Session, error) {
	if raw == "" {
		return Session{}, ErrUnauthorized
	}
	rawRefresh, next, err := s.newRefreshToken(uuid.Nil)
	if err != nil {
		return Session{}, err
	}
	user, err := s.store.RotateRefreshToken(ctx, HashRefreshToken(raw), next)
	if err != nil {
		return Session{}, err
	}
	return s.sessionFor(user, rawRefresh)
}

func (s *Service) Logout(ctx context.Context, raw string) error {
	if raw == "" {
		return ErrUnauthorized
	}
	return s.store.RevokeRefreshToken(ctx, HashRefreshToken(raw))
}

func (s *Service) CurrentUser(ctx context.Context, id [16]byte) (User, error) {
	return s.store.GetUserByID(ctx, id)
}

func (s *Service) newRefreshToken(userID uuid.UUID) (string, RefreshTokenInput, error) {
	raw, hash, err := NewRefreshToken()
	if err != nil {
		return "", RefreshTokenInput{}, err
	}
	return raw, RefreshTokenInput{ID: uuid.New(), UserID: userID, Hash: hash, ExpiresAt: s.now().UTC().Add(RefreshTokenLifetime)}, nil
}

func (s *Service) sessionFor(user User, refresh string) (Session, error) {
	access, err := s.tokens.NewAccessToken(user.ID)
	if err != nil {
		return Session{}, err
	}
	return Session{AccessToken: access, ExpiresIn: int(AccessTokenLifetime.Seconds()), RefreshToken: refresh, User: user}, nil
}

func normalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if len(email) == 0 || len(email) > 320 {
		return "", ErrInvalidCredentials
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || !strings.Contains(email, "@") {
		return "", ErrInvalidCredentials
	}
	return email, nil
}

func IsValidationError(err error) bool {
	return errors.Is(err, ErrInvalidPassword) || errors.Is(err, ErrInvalidCredentials)
}
