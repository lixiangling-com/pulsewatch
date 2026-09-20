package monitor

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

func TestRepositoryEnforcesOwnershipLifecycleAndSoftDelete(t *testing.T) {
	pool := newMonitorTestPool(t)
	ctx := context.Background()
	ownerID := insertMonitorTestUser(t, pool, "owner@example.com")
	otherID := insertMonitorTestUser(t, pool, "other@example.com")
	service := NewService(NewRepository(pool))
	created, err := service.Create(ctx, ownerID, validCreate("API"))
	if err != nil {
		t.Fatal(err)
	}

	newName := "Stolen"
	for name, operation := range map[string]func() error{
		"get": func() error { _, err := service.Get(ctx, otherID, created.ID); return err },
		"update": func() error {
			_, err := service.Update(ctx, otherID, created.ID, UpdateInput{Name: &newName})
			return err
		},
		"pause":  func() error { _, err := service.Pause(ctx, otherID, created.ID); return err },
		"resume": func() error { _, err := service.Resume(ctx, otherID, created.ID); return err },
		"delete": func() error { return service.Delete(ctx, otherID, created.ID) },
	} {
		t.Run("other user "+name, func(t *testing.T) {
			if err := operation(); !errors.Is(err, ErrNotFound) {
				t.Fatalf("error = %v, want ErrNotFound", err)
			}
		})
	}

	newURL := "https://example.org/ready"
	updated, err := service.Update(ctx, ownerID, created.ID, UpdateInput{URL: &newURL})
	if err != nil || updated.ConfigVersion != 2 || updated.Status != StatusPending {
		t.Fatalf("update = %#v, %v", updated, err)
	}
	paused, err := service.Pause(ctx, ownerID, created.ID)
	if err != nil || paused.ConfigVersion != 3 || paused.Status != StatusPaused {
		t.Fatalf("pause = %#v, %v", paused, err)
	}
	repeatedPause, err := service.Pause(ctx, ownerID, created.ID)
	if err != nil || repeatedPause.ConfigVersion != 3 {
		t.Fatalf("repeated pause = %#v, %v", repeatedPause, err)
	}
	resumed, err := service.Resume(ctx, ownerID, created.ID)
	if err != nil || resumed.ConfigVersion != 4 || resumed.Status != StatusPending {
		t.Fatalf("resume = %#v, %v", resumed, err)
	}
	repeatedResume, err := service.Resume(ctx, ownerID, created.ID)
	if err != nil || repeatedResume.ConfigVersion != 4 {
		t.Fatalf("repeated resume = %#v, %v", repeatedResume, err)
	}

	if err := service.Delete(ctx, ownerID, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Get(ctx, ownerID, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get after delete error = %v", err)
	}
	if err := service.Delete(ctx, ownerID, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("repeated delete error = %v", err)
	}
	var version int
	var deletedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT config_version, deleted_at FROM monitors WHERE id = $1`, created.ID).Scan(&version, &deletedAt); err != nil {
		t.Fatal(err)
	}
	if version != 5 || deletedAt == nil {
		t.Fatalf("soft delete state = version %d, deleted_at %v", version, deletedAt)
	}
}

func TestRepositorySerializesConcurrentLimitCheck(t *testing.T) {
	pool := newMonitorTestPool(t)
	ctx := context.Background()
	userID := insertMonitorTestUser(t, pool, "limit@example.com")
	service := NewService(NewRepository(pool))
	for index := 0; index < MaxPerUser-1; index++ {
		if _, err := service.Create(ctx, userID, validCreate("Monitor")); err != nil {
			t.Fatalf("seed monitor %d: %v", index, err)
		}
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var done sync.WaitGroup
	for range 2 {
		done.Add(1)
		go func() {
			defer done.Done()
			<-start
			_, err := service.Create(ctx, userID, validCreate("Concurrent"))
			results <- err
		}()
	}
	close(start)
	done.Wait()
	close(results)

	var succeeded, limited int
	for err := range results {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrLimit):
			limited++
		default:
			t.Fatalf("unexpected create error: %v", err)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM monitors WHERE user_id = $1 AND deleted_at IS NULL`, userID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if succeeded != 1 || limited != 1 || count != MaxPerUser {
		t.Fatalf("succeeded=%d limited=%d count=%d", succeeded, limited, count)
	}
}

func newMonitorTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set; skipping PostgreSQL monitor integration test")
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
	schema := "day06_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		admin.Close()
		t.Fatalf("create temporary schema: %v", err)
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if _, err := admin.Exec(cleanupCtx, `DROP SCHEMA "`+schema+`" CASCADE`); err != nil {
			t.Errorf("drop schema: %v", err)
		}
		admin.Close()
	})
	data, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", "00001_init.sql"))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(data), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatal("migration must contain one goose Down marker")
	}
	if _, err := pool.Exec(ctx, strings.TrimPrefix(parts[0], "-- +goose Up")); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	return pool
}

func insertMonitorTestUser(t *testing.T, pool *pgxpool.Pool, email string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(context.Background(), `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, 'test-hash')`, id, email); err != nil {
		t.Fatal(err)
	}
	return id
}
