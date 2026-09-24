package notification

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
)

type Dispatcher struct {
	store    notificationStore
	client   Enqueuer
	interval time.Duration
	limit    int
	logger   *slog.Logger
}
type Enqueuer interface {
	EnqueueContext(context.Context, *asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
}
type notificationStore interface {
	PendingIDs(context.Context, int) ([]uuid.UUID, error)
	PrepareRetry(context.Context, uuid.UUID) error
	MarkQueued(context.Context, uuid.UUID) error
}
type Store struct{ queries *sqlc.Queries }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{queries: sqlc.New(pool)} }
func NewDispatcher(store notificationStore, client Enqueuer, interval time.Duration, limit int, logger *slog.Logger) *Dispatcher {
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
	ids, err := d.store.PendingIDs(ctx, d.limit)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := d.store.PrepareRetry(ctx, id); err != nil {
			return err
		}
		task, err := NewMailTask(id)
		if err != nil {
			return err
		}
		_, err = d.client.EnqueueContext(ctx, task, asynq.TaskID(id.String()))
		if err != nil && !errors.Is(err, asynq.ErrTaskIDConflict) && !errors.Is(err, asynq.ErrDuplicateTask) {
			return err
		}
		if err := d.store.MarkQueued(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) PendingIDs(ctx context.Context, limit int) ([]uuid.UUID, error) {
	rows, err := s.queries.ListPendingEmailNotificationIDs(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, uuid.UUID(row.Bytes))
	}
	return ids, nil
}
func (s *Store) MarkQueued(ctx context.Context, id uuid.UUID) error {
	_, err := s.queries.MarkNotificationQueued(ctx, pgtype.UUID{Bytes: id, Valid: true})
	return err
}
func (s *Store) PrepareRetry(ctx context.Context, id uuid.UUID) error {
	_, err := s.queries.PrepareNotificationRetry(ctx, pgtype.UUID{Bytes: id, Valid: true})
	return err
}
func (d *Dispatcher) tick(ctx context.Context) {
	if err := d.DispatchPending(ctx); err != nil && ctx.Err() == nil {
		d.logger.Error("notification dispatch failed", slog.String("error", err.Error()))
	}
}
