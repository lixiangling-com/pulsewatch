package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeStore struct {
	mu    sync.Mutex
	calls int
	limit int
	ids   []uuid.UUID
}

func (s *fakeStore) CreateDueRuns(_ context.Context, limit int) ([]uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.limit = limit
	return s.ids, nil
}

func (s *fakeStore) snapshot() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls, s.limit
}

func TestRunScansImmediatelyAndStopsWithContext(t *testing.T) {
	store := &fakeStore{ids: []uuid.UUID{uuid.New()}}
	ctx, cancel := context.WithCancel(context.Background())
	s := New(store, time.Hour, 7, nil)
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	deadline := time.After(time.Second)
	for calls, _ := store.snapshot(); calls == 0; calls, _ = store.snapshot() {
		select {
		case <-deadline:
			t.Fatal("scheduler did not scan immediately")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if _, limit := store.snapshot(); limit != 7 {
		t.Fatalf("batch limit = %d", limit)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop")
	}
}
