package monitor

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	StatusPending = "pending"
	StatusPaused  = "paused"
	MaxPerUser    = 20
)

var (
	ErrNotFound = errors.New("monitor not found")
	ErrLimit    = errors.New("monitor limit reached")
)

type ValidationError struct {
	Fields map[string][]string
}

func (e *ValidationError) Error() string { return "monitor validation failed" }

type Monitor struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	Name            string
	URL             string
	IntervalMinutes int
	ExpectedStatus  int
	Status          string
	ConfigVersion   int
	NextCheckAt     time.Time
	LastCheckedAt   *time.Time
	LastLatencyMS   *int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CreateInput struct {
	Name            string
	URL             string
	IntervalMinutes int
	ExpectedStatus  int
}

type UpdateInput struct {
	Name            *string
	URL             *string
	IntervalMinutes *int
	ExpectedStatus  *int
}

type Page struct {
	Items  []Monitor
	Number int
	Size   int
	Total  int64
}

type MutateFunc func(Monitor) (Monitor, error)

type Store interface {
	Create(context.Context, uuid.UUID, Monitor) (Monitor, error)
	List(context.Context, uuid.UUID, int, int) ([]Monitor, int64, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (Monitor, error)
	Mutate(context.Context, uuid.UUID, uuid.UUID, MutateFunc) (Monitor, error)
	Delete(context.Context, uuid.UUID, uuid.UUID) error
}
