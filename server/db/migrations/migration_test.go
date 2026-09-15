package migrations

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSchemaConstraintsAndMigrationRoundTrip(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set; skipping PostgreSQL integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping PostgreSQL: %v", err)
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire PostgreSQL connection: %v", err)
	}
	defer conn.Release()

	schema := "day03_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := conn.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatalf("create temporary schema: %v", err)
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), `DROP SCHEMA "`+schema+`" CASCADE`)
	}()
	if _, err := conn.Exec(ctx, `SET search_path TO "`+schema+`", public`); err != nil {
		t.Fatalf("set search path: %v", err)
	}

	up, down := migrationSQL(t)
	if _, err := conn.Exec(ctx, up); err != nil {
		t.Fatalf("migration up: %v", err)
	}

	userID := uuid.New()
	if _, err := conn.Exec(ctx, `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)`, userID, "Demo@example.com", "bcrypt-hash"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	assertExecFails(t, ctx, conn, `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)`, uuid.New(), "demo@EXAMPLE.com", "another")

	monitorID := uuid.New()
	assertExecFails(t, ctx, conn, `INSERT INTO monitors (id, user_id, name, url, interval_minutes, expected_status) VALUES ($1, $2, $3, $4, $5, $6)`, monitorID, userID, "bad interval", "https://example.com", 2, 200)
	assertExecFails(t, ctx, conn, `INSERT INTO monitors (id, user_id, name, url, interval_minutes, expected_status, status) VALUES ($1, $2, $3, $4, $5, $6, $7)`, uuid.New(), userID, "bad status", "https://example.com", 5, 200, "unknown")
	if _, err := conn.Exec(ctx, `INSERT INTO monitors (id, user_id, name, url, interval_minutes, expected_status) VALUES ($1, $2, $3, $4, $5, $6)`, monitorID, userID, "Homepage", "https://example.com", 5, 200); err != nil {
		t.Fatalf("insert monitor: %v", err)
	}
	assertExecFails(t, ctx, conn, `INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at) VALUES ($1, $2, $3, now())`, uuid.New(), uuid.New(), make([]byte, 32))

	incidentID := uuid.New()
	if _, err := conn.Exec(ctx, `INSERT INTO incidents (id, monitor_id, opened_at, error_code) VALUES ($1, $2, now(), $3)`, incidentID, monitorID, "timeout"); err != nil {
		t.Fatalf("insert incident: %v", err)
	}
	assertExecFails(t, ctx, conn, `INSERT INTO incidents (id, monitor_id, opened_at, error_code) VALUES ($1, $2, now(), $3)`, uuid.New(), monitorID, "timeout")
	if _, err := conn.Exec(ctx, `INSERT INTO notifications (id, user_id, monitor_id, incident_id, type, dedupe_key, title, body) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, uuid.New(), userID, monitorID, incidentID, "incident_opened", "incident:test:opened", "故障", "请求超时"); err != nil {
		t.Fatalf("insert notification: %v", err)
	}
	assertExecFails(t, ctx, conn, `INSERT INTO notifications (id, user_id, monitor_id, incident_id, type, dedupe_key, title, body) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, uuid.New(), userID, monitorID, incidentID, "incident_opened", "incident:test:opened", "重复", "重复")

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin rollback transaction: %v", err)
	}
	rolledBackUser := uuid.New()
	if _, err := tx.Exec(ctx, `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)`, rolledBackUser, "rollback@example.com", "hash"); err != nil {
		t.Fatalf("insert transaction row: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback transaction: %v", err)
	}
	var count int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM users WHERE id = $1`, rolledBackUser).Scan(&count); err != nil {
		t.Fatalf("check rollback row: %v", err)
	}
	if count != 0 {
		t.Fatalf("rollback row still exists: count=%d", count)
	}

	if _, err := conn.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
		t.Fatalf("delete user for cascade check: %v", err)
	}
	for _, table := range []string{"refresh_tokens", "monitors", "incidents", "notifications"} {
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&count); err != nil {
			t.Fatalf("count %s after cascade: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("cascade left %d rows in %s", count, table)
		}
	}

	if _, err := conn.Exec(ctx, down); err != nil {
		t.Fatalf("migration down: %v", err)
	}
	if _, err := conn.Exec(ctx, up); err != nil {
		t.Fatalf("migration up after down: %v", err)
	}
}

func migrationSQL(t *testing.T) (string, string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("00001_init.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	parts := strings.Split(string(data), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatal("migration must contain one goose Down marker")
	}
	up := strings.TrimPrefix(parts[0], "-- +goose Up")
	return up, parts[1]
}

func assertExecFails(t *testing.T, ctx context.Context, conn *pgxpool.Conn, query string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(ctx, query, args...); err == nil {
		t.Fatalf("expected statement to fail: %s", query)
	}
}
