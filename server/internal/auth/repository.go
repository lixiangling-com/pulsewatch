package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lixiangling-com/pulsewatch/server/db/sqlc"
)

type Store interface {
	Register(context.Context, PasswordUser, RefreshTokenInput) (User, error)
	GetUserByEmail(context.Context, string) (PasswordUser, error)
	GetUserByID(context.Context, [16]byte) (User, error)
	CreateRefreshToken(context.Context, RefreshTokenInput) error
	RotateRefreshToken(context.Context, [32]byte, RefreshTokenInput) (User, error)
	RevokeRefreshToken(context.Context, [32]byte) error
}

type Repository struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
	now     func() time.Time
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, queries: sqlc.New(pool), now: time.Now}
}

func (r *Repository) Register(ctx context.Context, user PasswordUser, refresh RefreshTokenInput) (User, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := r.queries.WithTx(tx)
	created, err := queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:           toPGUUID(user.ID),
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrEmailAlreadyRegistered
		}
		return User{}, err
	}
	if err := createRefreshToken(ctx, queries, refresh); err != nil {
		return User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return userFromModel(created), nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (PasswordUser, error) {
	user, err := r.queries.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return PasswordUser{}, ErrUserNotFound
	}
	if err != nil {
		return PasswordUser{}, err
	}
	return passwordUserFromModel(user), nil
}

func (r *Repository) GetUserByID(ctx context.Context, id [16]byte) (User, error) {
	user, err := r.queries.GetUserByID(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, err
	}
	return userFromModel(user), nil
}

func (r *Repository) CreateRefreshToken(ctx context.Context, refresh RefreshTokenInput) error {
	return createRefreshToken(ctx, r.queries, refresh)
}

func (r *Repository) RotateRefreshToken(ctx context.Context, oldHash [32]byte, next RefreshTokenInput) (User, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := r.queries.WithTx(tx)
	current, err := queries.GetRefreshTokenForUpdate(ctx, oldHash[:])
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, err
	}
	if current.RevokedAt.Valid || !current.ExpiresAt.Valid || !current.ExpiresAt.Time.After(r.now()) {
		return User{}, ErrUnauthorized
	}
	user, err := queries.GetUserByID(ctx, current.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, err
	}
	changed, err := queries.RevokeRefreshToken(ctx, current.ID)
	if err != nil || changed != 1 {
		return User{}, ErrUnauthorized
	}
	next.UserID = current.UserID.Bytes
	if err := createRefreshToken(ctx, queries, next); err != nil {
		return User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return userFromModel(user), nil
}

func (r *Repository) RevokeRefreshToken(ctx context.Context, hash [32]byte) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := r.queries.WithTx(tx)
	current, err := queries.GetRefreshTokenForUpdate(ctx, hash[:])
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && current.RevokedAt.Valid) {
		return ErrUnauthorized
	}
	if err != nil {
		return err
	}
	changed, err := queries.RevokeRefreshToken(ctx, current.ID)
	if err != nil || changed != 1 {
		return ErrUnauthorized
	}
	return tx.Commit(ctx)
}

func createRefreshToken(ctx context.Context, queries *sqlc.Queries, token RefreshTokenInput) error {
	_, err := queries.CreateRefreshToken(ctx, sqlc.CreateRefreshTokenParams{
		ID:        toPGUUID(token.ID),
		UserID:    toPGUUID(token.UserID),
		TokenHash: token.Hash[:],
		ExpiresAt: pgtype.Timestamptz{Time: token.ExpiresAt.UTC(), Valid: true},
	})
	return err
}

func toPGUUID(value [16]byte) pgtype.UUID {
	return pgtype.UUID{Bytes: value, Valid: true}
}

func userFromModel(value sqlc.User) User {
	return User{ID: value.ID.Bytes, Email: value.Email, CreatedAt: value.CreatedAt.Time, UpdatedAt: value.UpdatedAt.Time}
}

func passwordUserFromModel(value sqlc.User) PasswordUser {
	return PasswordUser{User: userFromModel(value), PasswordHash: value.PasswordHash}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
