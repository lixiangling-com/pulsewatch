package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lixiangling-com/pulsewatch/server/db/sqlc"
)

type Store interface {
	CreateDueRuns(context.Context, int) ([]uuid.UUID, error)
}

type Scheduler struct {
	store    Store
	interval time.Duration
	limit    int
	logger   *slog.Logger
}

func New(store Store, interval time.Duration, limit int, logger *slog.Logger) *Scheduler {
	if interval <= 0 {
		interval = time.Second
	}
	if limit <= 0 {
		limit = 20
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{store: store, interval: interval, limit: limit, logger: logger}
}

func (s *Scheduler) Run(ctx context.Context) {
	s.tick(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.tick(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	runIDs, err := s.store.CreateDueRuns(ctx, s.limit)
	if err != nil {
		if ctx.Err() == nil {
			s.logger.Error("scheduler scan failed", slog.String("error", err.Error()))
		}
		return
	}
	if len(runIDs) > 0 {
		s.logger.Info("scheduled check runs", slog.Int("count", len(runIDs)))
	}
}

type Repository struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: sqlc.New(pool)}
}

func (r *Repository) CreateDueRuns(ctx context.Context, limit int) ([]uuid.UUID, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := r.queries.WithTx(tx)
	monitors, err := queries.ListDueMonitorsForUpdate(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	runIDs := make([]uuid.UUID, 0, len(monitors))
	for _, monitor := range monitors {
		runID := uuid.New()
		_, err := queries.CreateCheckRun(ctx, sqlc.CreateCheckRunParams{
			ID:            pgUUID(runID),
			MonitorID:     monitor.ID,
			ConfigVersion: monitor.ConfigVersion,
			Status:        "queued",
			ScheduledAt:   monitor.NextCheckAt,
		})
		if err != nil {
			return nil, err
		}
		next := monitor.NextCheckAt.Time.Add(time.Duration(monitor.IntervalMinutes) * time.Minute)
		changed, err := queries.AdvanceMonitorNextCheck(ctx, sqlc.AdvanceMonitorNextCheckParams{
			NextCheckAt: pgtype.Timestamptz{Time: next, Valid: true},
			ID:          monitor.ID,
		})
		if err != nil {
			return nil, err
		}
		if changed != 1 {
			return nil, pgx.ErrNoRows
		}
		runIDs = append(runIDs, runID)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return runIDs, nil
}

func pgUUID(value uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: value, Valid: true}
}
