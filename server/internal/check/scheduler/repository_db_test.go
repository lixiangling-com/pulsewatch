package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCreateDueRunsConcurrentSchedulersCreateOneRun(t *testing.T) {
	pool, ctx := newIntegrationPool(t)
	userID, monitorID := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id,email,password_hash) VALUES ($1,$2,$3)`, userID, "scheduler@example.com", "hash"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO monitors (id,user_id,name,url,interval_minutes,next_check_at) VALUES ($1,$2,$3,$4,5,now()-interval '1 minute')`, monitorID, userID, "scheduler", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool)
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := repo.CreateDueRuns(ctx, 20); results <- err }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	var runs, nextCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM check_runs WHERE monitor_id=$1`, monitorID).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM monitors WHERE id=$1 AND next_check_at > now()`, monitorID).Scan(&nextCount); err != nil {
		t.Fatal(err)
	}
	if runs != 1 || nextCount != 1 {
		t.Fatalf("runs=%d next advanced rows=%d", runs, nextCount)
	}
}

func TestCreateDueRunsSkipsPausedDeletedAndFutureMonitors(t *testing.T) {
	pool, ctx := newIntegrationPool(t)
	userID := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id,email,password_hash) VALUES ($1,$2,$3)`, userID, "scheduler-scenarios@example.com", "hash"); err != nil {
		t.Fatal(err)
	}
	type monitorCase struct {
		id      uuid.UUID
		status  string
		deleted bool
		due     string
		version int
	}
	cases := []monitorCase{
		{id: uuid.New(), status: "pending", due: "now() - interval '2 minutes'", version: 7},
		{id: uuid.New(), status: "paused", due: "now() - interval '2 minutes'", version: 2},
		{id: uuid.New(), status: "pending", deleted: true, due: "now() - interval '2 minutes'", version: 3},
		{id: uuid.New(), status: "pending", due: "now() + interval '1 hour'", version: 4},
	}
	for _, item := range cases {
		deletedAt := "NULL"
		if item.deleted {
			deletedAt = "now()"
		}
		query := `INSERT INTO monitors (id,user_id,name,url,interval_minutes,status,config_version,next_check_at,deleted_at)
VALUES ($1,$2,$3,$4,5,$5,$6,` + item.due + `,` + deletedAt + `)`
		if _, err := pool.Exec(ctx, query, item.id, userID, item.id.String(), "https://example.com", item.status, item.version); err != nil {
			t.Fatal(err)
		}
	}
	runIDs, err := NewRepository(pool).CreateDueRuns(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(runIDs) != 1 {
		t.Fatalf("created run IDs = %d, want 1", len(runIDs))
	}
	var configVersion int
	var scheduledAt, nextCheckAt time.Time
	if err := pool.QueryRow(ctx, `SELECT r.config_version, r.scheduled_at, m.next_check_at
FROM check_runs r JOIN monitors m ON m.id = r.monitor_id WHERE r.id = $1`, runIDs[0]).Scan(&configVersion, &scheduledAt, &nextCheckAt); err != nil {
		t.Fatal(err)
	}
	if configVersion != cases[0].version {
		t.Fatalf("config_version = %d, want %d", configVersion, cases[0].version)
	}
	if !nextCheckAt.After(scheduledAt) || nextCheckAt.Sub(scheduledAt) != 5*time.Minute {
		t.Fatalf("next_check_at = %s, scheduled_at = %s", nextCheckAt, scheduledAt)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM check_runs r JOIN monitors m ON m.id = r.monitor_id WHERE m.user_id = $1`, userID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("check run count = %d, want 1", count)
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
	schema := "day08_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
		up := strings.Split(string(data), "-- +goose Down")[0]
		up = strings.TrimPrefix(up, "-- +goose Up")
		if _, err := pool.Exec(ctx, up); err != nil {
			t.Fatal(err)
		}
	}
	return pool, ctx
}
