package auth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// authSchema is the search_path for the throwaway schema every test in this
// file runs against. Keeping it on the connection config means every pooled
// connection sees the same isolated tables.
func newAuthTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set; skipping PostgreSQL auth integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	if err := admin.Ping(ctx); err != nil {
		admin.Close()
		t.Fatalf("ping PostgreSQL: %v", err)
	}

	schema := "day04_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		admin.Close()
		t.Fatalf("create temporary schema: %v", err)
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		admin.Close()
		t.Fatalf("parse database URL: %v", err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		admin.Close()
		t.Fatalf("connect to temporary schema: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if _, err := admin.Exec(cleanupCtx, `DROP SCHEMA "`+schema+`" CASCADE`); err != nil {
			t.Errorf("drop temporary schema %s: %v", schema, err)
		}
		admin.Close()
	})

	if _, err := pool.Exec(ctx, migrationUp(t)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	return pool
}

// migrationUp reads the existing Day03 migration so the auth integration test
// runs against the same schema the API ships with.
func migrationUp(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", "00001_init.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	parts := strings.Split(string(data), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatal("migration must contain one goose Down marker")
	}
	return strings.TrimPrefix(parts[0], "-- +goose Up")
}

func newAuthTestService(t *testing.T) (*Service, *pgxpool.Pool) {
	t.Helper()
	pool := newAuthTestPool(t)
	manager, err := NewTokenManager("this-is-a-local-development-secret-at-least-32-bytes", "pulsewatch")
	if err != nil {
		t.Fatalf("NewTokenManager() error = %v", err)
	}
	return NewService(NewRepository(pool), manager), pool
}

func TestRepositoryRegisterStoresOnlyRefreshTokenDigest(t *testing.T) {
	ctx := context.Background()
	service, pool := newAuthTestService(t)

	session, err := service.Register(ctx, "Demo@Example.com ", "correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if session.User.Email != "demo@example.com" {
		t.Fatalf("registered email = %q", session.User.Email)
	}

	var storedHash []byte
	if err := pool.QueryRow(ctx, `SELECT token_hash FROM refresh_tokens WHERE user_id = $1`, session.User.ID).Scan(&storedHash); err != nil {
		t.Fatalf("read stored refresh token: %v", err)
	}
	if want := HashRefreshToken(session.RefreshToken); string(storedHash) != string(want[:]) {
		t.Fatal("stored token_hash is not the SHA-256 digest of the issued refresh token")
	}
	if strings.Contains(string(storedHash), session.RefreshToken) {
		t.Fatal("database stored the raw refresh token value")
	}

	var passwordHash string
	if err := pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, session.User.ID).Scan(&passwordHash); err != nil {
		t.Fatalf("read stored password hash: %v", err)
	}
	if passwordHash == "correct-horse-battery-staple" || !CheckPassword(passwordHash, "correct-horse-battery-staple") {
		t.Fatal("database did not store a bcrypt digest of the password")
	}
}

func TestRepositoryRegisterRejectsDuplicateEmailAndRollsBackAccount(t *testing.T) {
	ctx := context.Background()
	service, pool := newAuthTestService(t)

	if _, err := service.Register(ctx, "demo@example.com", "correct-horse-battery-staple"); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}
	if _, err := service.Register(ctx, "DEMO@Example.com", "another-long-password"); !errors.Is(err, ErrEmailAlreadyRegistered) {
		t.Fatalf("duplicate Register() error = %v", err)
	}

	var users, tokens int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&users); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM refresh_tokens`).Scan(&tokens); err != nil {
		t.Fatalf("count refresh tokens: %v", err)
	}
	if users != 1 || tokens != 1 {
		t.Fatalf("failed registration left rows behind: users=%d refresh_tokens=%d", users, tokens)
	}
}

func TestRepositoryLoginRejectsUnknownEmailAndWrongPasswordEqually(t *testing.T) {
	ctx := context.Background()
	service, _ := newAuthTestService(t)

	if _, err := service.Register(ctx, "demo@example.com", "correct-horse-battery-staple"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if _, err := service.Login(ctx, "demo@example.com", "wrong-password-entirely"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password error = %v", err)
	}
	if _, err := service.Login(ctx, "missing@example.com", "correct-horse-battery-staple"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("unknown email error = %v", err)
	}
	if _, err := service.Login(ctx, "demo@example.com", "correct-horse-battery-staple"); err != nil {
		t.Fatalf("valid Login() error = %v", err)
	}
}

func TestRepositoryRotateRefreshTokenRevokesTheOldRow(t *testing.T) {
	ctx := context.Background()
	service, pool := newAuthTestService(t)

	registered, err := service.Register(ctx, "demo@example.com", "correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	rotated, err := service.Refresh(ctx, registered.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if rotated.RefreshToken == registered.RefreshToken {
		t.Fatal("Refresh() reused the previous refresh token")
	}

	oldHash := HashRefreshToken(registered.RefreshToken)
	var revoked *time.Time
	if err := pool.QueryRow(ctx, `SELECT revoked_at FROM refresh_tokens WHERE token_hash = $1`, oldHash[:]).Scan(&revoked); err != nil {
		t.Fatalf("read rotated row: %v", err)
	}
	if revoked == nil {
		t.Fatal("rotated refresh token row was not revoked")
	}

	if _, err := service.Refresh(ctx, registered.RefreshToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("reused refresh token error = %v", err)
	}
	if err := service.Logout(ctx, rotated.RefreshToken); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if _, err := service.Refresh(ctx, rotated.RefreshToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("refresh after logout error = %v", err)
	}
}

func TestRepositoryConcurrentRefreshOnlyOneSucceeds(t *testing.T) {
	ctx := context.Background()
	service, pool := newAuthTestService(t)

	registered, err := service.Register(ctx, "demo@example.com", "correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	const attempts = 8
	results := make(chan error, attempts)
	var start sync.WaitGroup
	var done sync.WaitGroup
	start.Add(1)
	for range attempts {
		done.Add(1)
		go func() {
			defer done.Done()
			start.Wait()
			_, refreshErr := service.Refresh(ctx, registered.RefreshToken)
			results <- refreshErr
		}()
	}
	start.Done()
	done.Wait()
	close(results)

	var succeeded int
	for err := range results {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrUnauthorized):
		default:
			t.Fatalf("unexpected concurrent refresh error: %v", err)
		}
	}
	if succeeded != 1 {
		t.Fatalf("concurrent refresh succeeded %d times, want exactly 1", succeeded)
	}

	// Exactly one live successor token may remain after the race.
	var live int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM refresh_tokens WHERE revoked_at IS NULL`).Scan(&live); err != nil {
		t.Fatalf("count live refresh tokens: %v", err)
	}
	if live != 1 {
		t.Fatalf("live refresh tokens = %d, want 1", live)
	}
}
