package consumer

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lixiangling-com/pulsewatch/server/db/sqlc"
	checktask "github.com/lixiangling-com/pulsewatch/server/internal/check"
)

var ErrRunNotFound = errors.New("check run not found")

type Processor interface {
	Process(context.Context, uuid.UUID) error
}

type Consumer struct {
	processor Processor
	logger    *slog.Logger
}

func New(processor Processor, logger *slog.Logger) *Consumer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Consumer{processor: processor, logger: logger}
}

func (c *Consumer) HandleCheckRun(ctx context.Context, task *asynq.Task) error {
	payload, err := checktask.ParseCheckRunPayload(task.Payload())
	if err != nil {
		return err
	}
	if err := c.processor.Process(ctx, payload.RunID); err != nil {
		if errors.Is(err, ErrRunNotFound) {
			c.logger.Warn("check run no longer exists", slog.String("run_id", payload.RunID.String()))
			return nil
		}
		return err
	}
	return nil
}

type Repository struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: sqlc.New(pool)}
}

func (r *Repository) Process(ctx context.Context, id uuid.UUID) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := r.queries.WithTx(tx)
	run, err := queries.GetCheckRunForProcessing(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrRunNotFound
	}
	if err != nil {
		return err
	}
	if run.Status == "succeeded" || run.Status == "failed" || run.Status == "cancelled" {
		return tx.Commit(ctx)
	}
	if _, err := queries.MarkCheckRunRunning(ctx, run.ID); err != nil {
		return err
	}

	status := "succeeded"
	var code, summary pgtype.Text
	switch {
	case run.MonitorDeletedAt.Valid:
		status, code, summary = cancellation("monitor_deleted", "monitor was deleted before processing")
	case run.MonitorStatus == "paused":
		status, code, summary = cancellation("monitor_paused", "monitor was paused before processing")
	case run.ConfigVersion != run.MonitorConfigVersion:
		status, code, summary = cancellation("stale_config", "monitor configuration changed before processing")
	}
	if _, err := queries.CompleteCheckRun(ctx, sqlc.CompleteCheckRunParams{
		Status: status, ErrorCode: code, ErrorSummary: summary, ID: run.ID,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func cancellation(codeValue, summaryValue string) (string, pgtype.Text, pgtype.Text) {
	return "cancelled", pgtype.Text{String: codeValue, Valid: true}, pgtype.Text{String: summaryValue, Valid: true}
}
