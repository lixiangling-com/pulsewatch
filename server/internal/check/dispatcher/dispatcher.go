package dispatcher

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lixiangling-com/pulsewatch/server/db/sqlc"
	checktask "github.com/lixiangling-com/pulsewatch/server/internal/check"
)

type Store interface {
	PendingRunIDs(context.Context, int) ([]uuid.UUID, error)
	MarkEnqueued(context.Context, uuid.UUID) error
}

type Enqueuer interface {
	EnqueueContext(context.Context, *asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
}

type Dispatcher struct {
	store    Store
	client   Enqueuer
	interval time.Duration
	limit    int
	logger   *slog.Logger
}

func New(store Store, client Enqueuer, interval time.Duration, limit int, logger *slog.Logger) *Dispatcher {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if limit <= 0 {
		limit = 100
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Dispatcher{store: store, client: client, interval: interval, limit: limit, logger: logger}
}

func (d *Dispatcher) Run(ctx context.Context) {
	d.tick(ctx)
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			d.tick(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (d *Dispatcher) DispatchPending(ctx context.Context) error {
	runIDs, err := d.store.PendingRunIDs(ctx, d.limit)
	if err != nil {
		return err
	}
	for _, runID := range runIDs {
		task, err := checktask.NewCheckRunTask(runID)
		if err != nil {
			return err
		}
		_, err = d.client.EnqueueContext(ctx, task, asynq.TaskID(runID.String()))
		if err != nil && !errors.Is(err, asynq.ErrTaskIDConflict) && !errors.Is(err, asynq.ErrDuplicateTask) {
			return err
		}
		if err := d.store.MarkEnqueued(ctx, runID); err != nil {
			return err
		}
	}
	return nil
}

func (d *Dispatcher) tick(ctx context.Context) {
	if err := d.DispatchPending(ctx); err != nil && ctx.Err() == nil {
		d.logger.Error("check run dispatch failed", slog.String("error", err.Error()))
	}
}

type Repository struct {
	queries *sqlc.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{queries: sqlc.New(pool)}
}

func (r *Repository) PendingRunIDs(ctx context.Context, limit int) ([]uuid.UUID, error) {
	rows, err := r.queries.ListPendingCheckRunIDs(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, uuid.UUID(row.Bytes))
	}
	return ids, nil
}

func (r *Repository) MarkEnqueued(ctx context.Context, id uuid.UUID) error {
	_, err := r.queries.MarkCheckRunEnqueued(ctx, pgtype.UUID{Bytes: id, Valid: true})
	return err
}
