package consumer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lixiangling-com/pulsewatch/server/internal/check/checker"
)

type fixedChecker struct {
	result checker.Result
}

func (f fixedChecker) Check(context.Context, checker.Target) checker.Result { return f.result }

func TestProcessPersistsSuccessAndIsIdempotent(t *testing.T) {
	pool, ctx := newIntegrationPool(t)
	monitorID, runID := insertMonitorAndRun(t, pool, ctx, "pending", false, 1, 1)
	repo := NewRepository(pool, fixedChecker{checker.Result{Outcome: checker.OutcomeSuccess, StatusCode: 200, Latency: 17 * time.Millisecond}})
	if err := repo.Process(ctx, runID); err != nil {
		t.Fatal(err)
	}
	if err := repo.Process(ctx, runID); err != nil {
		t.Fatal(err)
	}
	assertRunState(t, pool, ctx, runID, monitorID, "succeeded", "")
	var monitorStatus string
	var statusCode int32
	var latency pgtype.Int8
	if err := pool.QueryRow(ctx, `SELECT m.status,r.status_code,r.latency_ms FROM monitors m JOIN check_runs r ON r.monitor_id=m.id WHERE r.id=$1`, runID).Scan(&monitorStatus, &statusCode, &latency); err != nil {
		t.Fatal(err)
	}
	if monitorStatus != "up" || statusCode != 200 || !latency.Valid || latency.Int64 != 17 {
		t.Fatalf("result not fully persisted: status=%s code=%d latency=%+v", monitorStatus, statusCode, latency)
	}
}

func TestProcessPersistsTwoFailureConfirmationAndBlockedDoesNotChangeMonitor(t *testing.T) {
	pool, ctx := newIntegrationPool(t)
	monitorID, firstRun := insertMonitorAndRun(t, pool, ctx, "pending", false, 1, 1)
	failure := fixedChecker{checker.Result{Outcome: checker.OutcomeTargetFailure, ErrorCode: "http_5xx", Summary: "target returned a server error", StatusCode: 500}}
	if err := NewRepository(pool, failure).Process(ctx, firstRun); err != nil {
		t.Fatal(err)
	}
	assertRunState(t, pool, ctx, firstRun, monitorID, "failed", "http_5xx")
	var monitorStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM monitors WHERE id=$1`, monitorID).Scan(&monitorStatus); err != nil {
		t.Fatal(err)
	}
	if monitorStatus != "confirming_down" {
		t.Fatalf("after first failure monitor status=%q", monitorStatus)
	}

	blockedRun := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO check_runs (id,monitor_id,config_version,status,scheduled_at) VALUES ($1,$2,1,'queued',now()+interval '1 minute')`, blockedRun, monitorID); err != nil {
		t.Fatal(err)
	}
	blocked := fixedChecker{checker.Result{Outcome: checker.OutcomeBlocked, ErrorCode: "blocked_address", Summary: "target resolves to a restricted address"}}
	if err := NewRepository(pool, blocked).Process(ctx, blockedRun); err != nil {
		t.Fatal(err)
	}
	assertRunState(t, pool, ctx, blockedRun, monitorID, "failed", "blocked_address")
	if err := pool.QueryRow(ctx, `SELECT status FROM monitors WHERE id=$1`, monitorID).Scan(&monitorStatus); err != nil {
		t.Fatal(err)
	}
	if monitorStatus != "confirming_down" {
		t.Fatalf("blocked result changed monitor status to %q", monitorStatus)
	}
}

func TestProcessCancelsInvalidatedRuns(t *testing.T) {
	tests := []struct {
		name           string
		status         string
		deleted        bool
		runVersion     int
		monitorVersion int
		wantCode       string
	}{
		{name: "paused", status: "paused", runVersion: 1, monitorVersion: 1, wantCode: "monitor_paused"},
		{name: "deleted", status: "pending", deleted: true, runVersion: 1, monitorVersion: 2, wantCode: "monitor_deleted"},
		{name: "stale config", status: "pending", runVersion: 1, monitorVersion: 2, wantCode: "stale_config"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, ctx := newIntegrationPool(t)
			monitorID, runID := insertMonitorAndRun(t, pool, ctx, tt.status, tt.deleted, tt.monitorVersion, tt.runVersion)
			if err := NewRepository(pool, fixedChecker{checker.Result{Outcome: checker.OutcomeSuccess, StatusCode: 200}}).Process(ctx, runID); err != nil {
				t.Fatal(err)
			}
			assertRunState(t, pool, ctx, runID, monitorID, "cancelled", tt.wantCode)
		})
	}
}

func insertMonitorAndRun(t *testing.T, pool *pgxpool.Pool, ctx context.Context, status string, deleted bool, monitorVersion, runVersion int) (uuid.UUID, uuid.UUID) {
	t.Helper()
	userID, monitorID, runID := uuid.New(), uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id,email,password_hash) VALUES ($1,$2,$3)`, userID, userID.String()+"@example.com", "hash"); err != nil {
		t.Fatal(err)
	}
	deletedAt := "NULL"
	if deleted {
		deletedAt = "now()"
	}
	query := `INSERT INTO monitors (id,user_id,name,url,interval_minutes,status,config_version,deleted_at)
VALUES ($1,$2,$3,$4,5,$5,$6,` + deletedAt + `)`
	if _, err := pool.Exec(ctx, query, monitorID, userID, monitorID.String(), "https://example.com", status, monitorVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO check_runs (id,monitor_id,config_version,status,scheduled_at) VALUES ($1,$2,$3,'queued',now())`, runID, monitorID, runVersion); err != nil {
		t.Fatal(err)
	}
	return monitorID, runID
}

func assertRunState(t *testing.T, pool *pgxpool.Pool, ctx context.Context, runID, monitorID uuid.UUID, wantStatus, wantCode string) {
	t.Helper()
	var status, code string
	if err := pool.QueryRow(ctx, `SELECT status, coalesce(error_code, '') FROM check_runs WHERE id=$1 AND monitor_id=$2`, runID, monitorID).Scan(&status, &code); err != nil {
		t.Fatal(err)
	}
	if status != wantStatus || code != wantCode {
		t.Fatalf("status=%q error_code=%q, want %q/%q", status, code, wantStatus, wantCode)
	}
}

func newIntegrationPool(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set; skipping PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	if err := admin.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	schema := "day08_consumer_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), `DROP SCHEMA "`+schema+`" CASCADE`) })
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for _, name := range []string{"00001_init.sql", "00002_day08_queue_index.sql"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "db", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		up := strings.TrimPrefix(strings.Split(string(data), "-- +goose Down")[0], "-- +goose Up")
		if _, err := pool.Exec(ctx, up); err != nil {
			t.Fatal(err)
		}
	}
	return pool, ctx
}
