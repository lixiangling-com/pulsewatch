package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/lixiangling-com/pulsewatch/server/internal/platform/httpx"
)

func TestPasswordHashDoesNotExposePassword(t *testing.T) {
	password := "correct-horse-battery-staple"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == password || !CheckPassword(hash, password) || CheckPassword(hash, "wrong-password") {
		t.Fatal("password hashing or comparison is unsafe")
	}
	if _, err := HashPassword("short"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("short password error = %v", err)
	}
}

func TestHTTPRegisterReturnsCookieAndStableErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &memoryStore{}
	manager, _ := NewTokenManager("this-is-a-local-development-secret-at-least-32-bytes", "pulsewatch")
	handler := NewHandler(NewService(store, manager), true)
	router := gin.New()
	router.Use(httpx.RequestID())
	router.POST("/register", handler.Register)

	request := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{"email":"demo@example.com","password":"correct-horse-battery-staple"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", "auth-test-1")
	record := httptest.NewRecorder()
	router.ServeHTTP(record, request)
	if record.Code != http.StatusCreated || !strings.Contains(record.Header().Get("Set-Cookie"), "refresh_token=") || !strings.Contains(record.Header().Get("Set-Cookie"), "HttpOnly") || !strings.Contains(record.Header().Get("Set-Cookie"), "SameSite=Lax") || !strings.Contains(record.Header().Get("Set-Cookie"), "Expires=") {
		t.Fatalf("registration response = status=%d cookie=%q", record.Code, record.Header().Get("Set-Cookie"))
	}

	duplicate := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{"email":"DEMO@example.com","password":"correct-horse-battery-staple"}`))
	duplicate.Header.Set("Content-Type", "application/json")
	duplicate.Header.Set("X-Request-ID", "auth-test-2")
	record = httptest.NewRecorder()
	router.ServeHTTP(record, duplicate)
	if record.Code != http.StatusConflict || !strings.Contains(record.Body.String(), "EMAIL_ALREADY_REGISTERED") || !strings.Contains(record.Body.String(), "auth-test-2") {
		t.Fatalf("duplicate response = status=%d body=%s", record.Code, record.Body.String())
	}
}

func TestHTTPAuthMiddlewareRejectsInvalidAccessToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	manager, _ := NewTokenManager("this-is-a-local-development-secret-at-least-32-bytes", "pulsewatch")
	router := gin.New()
	router.Use(httpx.RequestID())
	router.GET("/me", manager.RequireUser(), func(c *gin.Context) { c.Status(http.StatusOK) })
	record := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	request.Header.Set("Authorization", "Bearer tampered")
	router.ServeHTTP(record, request)
	if record.Code != http.StatusUnauthorized || !strings.Contains(record.Body.String(), "UNAUTHORIZED") || !strings.Contains(record.Body.String(), "request_id") {
		t.Fatalf("middleware response = status=%d body=%s", record.Code, record.Body.String())
	}
}

func TestRefreshTokensAreRandomAndHashable(t *testing.T) {
	rawOne, hashOne, err := NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	rawTwo, hashTwo, err := NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if rawOne == rawTwo || hashOne == hashTwo || HashRefreshToken(rawOne) != hashOne {
		t.Fatal("refresh tokens must be random and their stored hash must match the raw cookie value")
	}
}

func TestAccessTokenRejectsTamperingWrongIssuerAndExpiry(t *testing.T) {
	manager, err := NewTokenManager("this-is-a-local-development-secret-at-least-32-bytes", "pulsewatch")
	if err != nil {
		t.Fatal(err)
	}
	userID := uuid.New()
	raw, err := manager.NewAccessToken(userID)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := manager.ParseAccessToken(raw); err != nil || got != userID {
		t.Fatalf("valid token = %s, %v", got, err)
	}
	if _, err := manager.ParseAccessToken(raw + "x"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("tampered token error = %v", err)
	}
	otherIssuer, _ := NewTokenManager("this-is-a-local-development-secret-at-least-32-bytes", "other")
	if _, err := otherIssuer.ParseAccessToken(raw); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("wrong issuer error = %v", err)
	}
	expiredClaims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		Issuer:    "pulsewatch",
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
	}
	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims).SignedString([]byte("this-is-a-local-development-secret-at-least-32-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ParseAccessToken(expired); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expired token error = %v", err)
	}
}

func TestServiceRefreshRotatesOnlyOnce(t *testing.T) {
	store := &memoryStore{}
	manager, _ := NewTokenManager("this-is-a-local-development-secret-at-least-32-bytes", "pulsewatch")
	service := NewService(store, manager)
	registered, err := service.Register(context.Background(), "Demo@Example.com", "correct-horse-battery-staple")
	if err != nil {
		t.Fatal(err)
	}
	if registered.User.Email != "demo@example.com" || registered.RefreshToken == "" {
		t.Fatalf("registration session = %#v", registered)
	}
	rotated, err := service.Refresh(context.Background(), registered.RefreshToken)
	if err != nil || rotated.RefreshToken == registered.RefreshToken {
		t.Fatalf("refresh = %#v, %v", rotated, err)
	}
	if _, err := service.Refresh(context.Background(), registered.RefreshToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("reused refresh token error = %v", err)
	}
	if err := service.Logout(context.Background(), rotated.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Refresh(context.Background(), rotated.RefreshToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("logged out refresh token error = %v", err)
	}
}

func TestServiceConcurrentRefreshOnlyOneSucceeds(t *testing.T) {
	store := &memoryStore{}
	manager, _ := NewTokenManager("this-is-a-local-development-secret-at-least-32-bytes", "pulsewatch")
	service := NewService(store, manager)
	registered, err := service.Register(context.Background(), "demo@example.com", "correct-horse-battery-staple")
	if err != nil {
		t.Fatal(err)
	}

	results := make(chan error, 2)
	for range 2 {
		go func() {
			_, refreshErr := service.Refresh(context.Background(), registered.RefreshToken)
			results <- refreshErr
		}()
	}
	var succeeded, rejected int
	for range 2 {
		err := <-results
		if err == nil {
			succeeded++
		} else if errors.Is(err, ErrUnauthorized) {
			rejected++
		} else {
			t.Fatalf("unexpected concurrent refresh error: %v", err)
		}
	}
	if succeeded != 1 || rejected != 1 {
		t.Fatalf("concurrent refresh outcomes: succeeded=%d rejected=%d", succeeded, rejected)
	}
}

type memoryStore struct {
	mu     sync.Mutex
	users  map[string]PasswordUser
	byID   map[uuid.UUID]User
	tokens map[[32]byte]RefreshTokenInput
}

func (s *memoryStore) Register(_ context.Context, user PasswordUser, token RefreshTokenInput) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.users == nil {
		s.users = map[string]PasswordUser{}
		s.byID = map[uuid.UUID]User{}
		s.tokens = map[[32]byte]RefreshTokenInput{}
	}
	if _, exists := s.users[user.Email]; exists {
		return User{}, ErrEmailAlreadyRegistered
	}
	user.CreatedAt = time.Now().UTC()
	user.UpdatedAt = user.CreatedAt
	s.users[user.Email] = user
	s.byID[user.ID] = user.User
	s.tokens[token.Hash] = token
	return user.User, nil
}

func (s *memoryStore) GetUserByEmail(_ context.Context, email string) (PasswordUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[email]
	if !ok {
		return PasswordUser{}, ErrUserNotFound
	}
	return user, nil
}

func (s *memoryStore) GetUserByID(_ context.Context, id [16]byte) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.byID[uuid.UUID(id)]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return user, nil
}

func (s *memoryStore) CreateRefreshToken(_ context.Context, token RefreshTokenInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token.Hash] = token
	return nil
}

func (s *memoryStore) RotateRefreshToken(_ context.Context, old [32]byte, next RefreshTokenInput) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.tokens[old]
	if !ok || !current.ExpiresAt.After(time.Now()) {
		return User{}, ErrUnauthorized
	}
	delete(s.tokens, old)
	next.UserID = current.UserID
	s.tokens[next.Hash] = next
	return s.byID[current.UserID], nil
}

func (s *memoryStore) RevokeRefreshToken(_ context.Context, hash [32]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tokens[hash]; !ok {
		return ErrUnauthorized
	}
	delete(s.tokens, hash)
	return nil
}
