package monitor

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lixiangling-com/pulsewatch/server/db/sqlc"
)

type Repository struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: sqlc.New(pool)}
}

func (r *Repository) Create(ctx context.Context, userID uuid.UUID, input Monitor) (Monitor, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Monitor{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := r.queries.WithTx(tx)
	if _, err := queries.LockUserForMonitorCreate(ctx, toPGUUID(userID)); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Monitor{}, ErrNotFound
		}
		return Monitor{}, err
	}
	count, err := queries.CountMonitorsByUser(ctx, toPGUUID(userID))
	if err != nil {
		return Monitor{}, err
	}
	if count >= MaxPerUser {
		return Monitor{}, ErrLimit
	}
	created, err := queries.CreateMonitor(ctx, sqlc.CreateMonitorParams{
		ID:              toPGUUID(input.ID),
		UserID:          toPGUUID(userID),
		Name:            input.Name,
		Url:             input.URL,
		IntervalMinutes: int32(input.IntervalMinutes),
		ExpectedStatus:  int32(input.ExpectedStatus),
	})
	if err != nil {
		return Monitor{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Monitor{}, err
	}
	return monitorFromModel(created), nil
}

func (r *Repository) List(ctx context.Context, userID uuid.UUID, page, pageSize int) ([]Monitor, int64, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := r.queries.WithTx(tx)
	total, err := queries.CountMonitorsByUser(ctx, toPGUUID(userID))
	if err != nil {
		return nil, 0, err
	}
	rows, err := queries.ListMonitorsByUser(ctx, sqlc.ListMonitorsByUserParams{
		UserID:     toPGUUID(userID),
		PageOffset: int32((page - 1) * pageSize),
		PageSize:   int32(pageSize),
	})
	if err != nil {
		return nil, 0, err
	}
	items := make([]Monitor, 0, len(rows))
	for _, row := range rows {
		items = append(items, monitorFromModel(row))
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *Repository) Get(ctx context.Context, userID, id uuid.UUID) (Monitor, error) {
	row, err := r.queries.GetMonitorByIDAndUser(ctx, sqlc.GetMonitorByIDAndUserParams{
		ID: toPGUUID(id), UserID: toPGUUID(userID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Monitor{}, ErrNotFound
	}
	if err != nil {
		return Monitor{}, err
	}
	return monitorFromModel(row), nil
}

func (r *Repository) Mutate(ctx context.Context, userID, id uuid.UUID, mutate MutateFunc) (Monitor, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Monitor{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := r.queries.WithTx(tx)
	row, err := queries.GetMonitorByIDAndUserForUpdate(ctx, sqlc.GetMonitorByIDAndUserForUpdateParams{
		ID: toPGUUID(id), UserID: toPGUUID(userID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Monitor{}, ErrNotFound
	}
	if err != nil {
		return Monitor{}, err
	}
	current := monitorFromModel(row)
	next, err := mutate(current)
	if err != nil {
		return Monitor{}, err
	}
	if next == current {
		return current, nil
	}
	updated, err := queries.UpdateMonitor(ctx, sqlc.UpdateMonitorParams{
		Name:            next.Name,
		Url:             next.URL,
		IntervalMinutes: int32(next.IntervalMinutes),
		ExpectedStatus:  int32(next.ExpectedStatus),
		Status:          next.Status,
		ConfigVersion:   int32(next.ConfigVersion),
		NextCheckAt:     pgtype.Timestamptz{Time: next.NextCheckAt.UTC(), Valid: true},
		ID:              toPGUUID(id),
		UserID:          toPGUUID(userID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Monitor{}, ErrNotFound
	}
	if err != nil {
		return Monitor{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Monitor{}, err
	}
	return monitorFromModel(updated), nil
}

func (r *Repository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	changed, err := r.queries.SoftDeleteMonitor(ctx, sqlc.SoftDeleteMonitorParams{
		ID: toPGUUID(id), UserID: toPGUUID(userID),
	})
	if err != nil {
		return err
	}
	if changed != 1 {
		return ErrNotFound
	}
	return nil
}

func toPGUUID(value uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: value, Valid: true}
}

func monitorFromModel(value sqlc.Monitor) Monitor {
	monitor := Monitor{
		ID:              uuid.UUID(value.ID.Bytes),
		UserID:          uuid.UUID(value.UserID.Bytes),
		Name:            value.Name,
		URL:             value.Url,
		IntervalMinutes: int(value.IntervalMinutes),
		ExpectedStatus:  int(value.ExpectedStatus),
		Status:          value.Status,
		ConfigVersion:   int(value.ConfigVersion),
		NextCheckAt:     value.NextCheckAt.Time,
		CreatedAt:       value.CreatedAt.Time,
		UpdatedAt:       value.UpdatedAt.Time,
	}
	if value.LastCheckedAt.Valid {
		lastCheckedAt := value.LastCheckedAt.Time
		monitor.LastCheckedAt = &lastCheckedAt
	}
	if value.LastLatencyMs.Valid {
		lastLatency := value.LastLatencyMs.Int64
		monitor.LastLatencyMS = &lastLatency
	}
	return monitor
}
