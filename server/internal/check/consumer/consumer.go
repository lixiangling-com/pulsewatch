package consumer

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lixiangling-com/pulsewatch/server/db/sqlc"
	checktask "github.com/lixiangling-com/pulsewatch/server/internal/check"
	"github.com/lixiangling-com/pulsewatch/server/internal/check/checker"
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
	checker checker.Checker
}

func NewRepository(pool *pgxpool.Pool, checkers ...checker.Checker) *Repository {
	var targetChecker checker.Checker
	if len(checkers) > 0 {
		targetChecker = checkers[0]
	}
	if targetChecker == nil {
		targetChecker = checker.NewHTTPChecker(false)
	}
	return &Repository{pool: pool, queries: sqlc.New(pool), checker: targetChecker}
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
	if run.Status == "succeeded" || run.Status == "failed" || run.Status == "cancelled" || run.Status == "running" {
		return tx.Commit(ctx)
	}
	if run.MonitorDeletedAt.Valid || run.MonitorStatus == "paused" || run.ConfigVersion != run.MonitorConfigVersion {
		code, summary := "stale_config", "monitor configuration changed before processing"
		if run.MonitorDeletedAt.Valid {
			code, summary = "monitor_deleted", "monitor was deleted before processing"
		} else if run.MonitorStatus == "paused" {
			code, summary = "monitor_paused", "monitor was paused before processing"
		}
		if _, err := queries.CompleteCheckRun(ctx, sqlc.CompleteCheckRunParams{Status: "cancelled", ErrorCode: pgtype.Text{String: code, Valid: true}, ErrorSummary: pgtype.Text{String: summary, Valid: true}, ID: run.ID}); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if _, err := queries.MarkCheckRunRunning(ctx, run.ID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	result := r.checker.Check(ctx, checker.Target{URL: run.Url, ExpectedStatus: int(run.ExpectedStatus)})
	if result.Outcome == checker.OutcomeSystemFailure {
		_, _ = r.queries.ResetCheckRunQueued(context.Background(), run.ID)
		return errors.New(result.ErrorCode)
	}
	if err := r.persistResult(ctx, run.ID, result); err != nil {
		_, _ = r.queries.ResetCheckRunQueued(context.Background(), run.ID)
		return err
	}
	return nil
}

func (r *Repository) persistResult(ctx context.Context, id pgtype.UUID, result checker.Result) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := r.queries.WithTx(tx)
	run, err := queries.GetCheckRunForProcessing(ctx, id)
	if err != nil {
		return err
	}
	if run.Status != "running" {
		return tx.Commit(ctx)
	}
	if run.MonitorDeletedAt.Valid || run.MonitorStatus == "paused" || run.ConfigVersion != run.MonitorConfigVersion {
		code, summary := "stale_config", "monitor configuration changed before processing"
		if run.MonitorDeletedAt.Valid {
			code, summary = "monitor_deleted", "monitor was deleted before processing"
		} else if run.MonitorStatus == "paused" {
			code, summary = "monitor_paused", "monitor was paused before processing"
		}
		_, err = queries.CompleteCheckRun(ctx, sqlc.CompleteCheckRunParams{Status: "cancelled", ErrorCode: pgtype.Text{String: code, Valid: true}, ErrorSummary: pgtype.Text{String: summary, Valid: true}, ID: id})
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	outcome := map[checker.Outcome]checktask.Outcome{checker.OutcomeSuccess: checktask.OutcomeSuccess, checker.OutcomeTargetFailure: checktask.OutcomeTargetFailure, checker.OutcomeBlocked: checktask.OutcomeBlocked}[result.Outcome]
	transition := checktask.NextState(run.MonitorStatus, outcome)
	runStatus := transition.RunStatus
	if result.Outcome == checker.OutcomeBlocked {
		runStatus = "failed"
	}
	if result.StatusCode < 100 || result.StatusCode > 599 {
		result.StatusCode = 0
	}
	code, summary := pgtype.Text{}, pgtype.Text{}
	if result.ErrorCode != "" {
		code = pgtype.Text{String: result.ErrorCode, Valid: true}
	}
	if result.Summary != "" {
		summary = pgtype.Text{String: result.Summary, Valid: true}
	}
	latency := int64(result.Latency / time.Millisecond)
	_, err = queries.PersistCheckRunResult(ctx, sqlc.PersistCheckRunResultParams{
		Status: runStatus, StatusCode: nullableInt(result.StatusCode), LatencyMs: pgtype.Int8{Int64: latency, Valid: true}, ErrorCode: code, ErrorSummary: summary, ID: id,
	})
	if err != nil {
		return err
	}
	if result.Outcome != checker.OutcomeBlocked {
		_, err = queries.UpdateMonitorCheckState(ctx, sqlc.UpdateMonitorCheckStateParams{Status: transition.State, LatencyMs: pgtype.Int8{Int64: latency, Valid: true}, ID: run.MonitorID})
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func nullableInt(value int) pgtype.Int4 {
	if value == 0 {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(value), Valid: true}
}

func cancellation(codeValue, summaryValue string) (string, pgtype.Text, pgtype.Text) {
	return "cancelled", pgtype.Text{String: codeValue, Valid: true}, pgtype.Text{String: summaryValue, Valid: true}
}
