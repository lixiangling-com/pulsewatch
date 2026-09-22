package dispatcher

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

type fakeStore struct {
	ids       []uuid.UUID
	marked    []uuid.UUID
	listError error
	markError error
}

func (s *fakeStore) PendingRunIDs(context.Context, int) ([]uuid.UUID, error) {
	return s.ids, s.listError
}
func (s *fakeStore) MarkEnqueued(_ context.Context, id uuid.UUID) error {
	if s.markError != nil {
		return s.markError
	}
	s.marked = append(s.marked, id)
	return nil
}

type fakeEnqueuer struct {
	task    *asynq.Task
	options []asynq.Option
	err     error
}

func (e *fakeEnqueuer) EnqueueContext(_ context.Context, task *asynq.Task, options ...asynq.Option) (*asynq.TaskInfo, error) {
	e.task, e.options = task, options
	return nil, e.err
}

func TestDispatchPendingUsesRunIDTaskAndMarksAfterDelivery(t *testing.T) {
	runID := uuid.New()
	store := &fakeStore{ids: []uuid.UUID{runID}}
	enqueuer := &fakeEnqueuer{}
	d := New(store, enqueuer, 0, 1, nil)
	if err := d.DispatchPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if enqueuer.task == nil || enqueuer.task.Type() != "check:run" {
		t.Fatalf("unexpected task: %#v", enqueuer.task)
	}
	if len(enqueuer.options) != 1 {
		t.Fatalf("enqueue options = %d, want 1", len(enqueuer.options))
	}
	option := enqueuer.options[0]
	if option.Type() != asynq.TaskIDOpt || option.Value() != runID.String() {
		t.Fatalf("task id option = %s, want %q", option.String(), runID.String())
	}
	if len(store.marked) != 1 || store.marked[0] != runID {
		t.Fatalf("marked = %#v", store.marked)
	}
}

func TestDispatchPendingKeepsRunWhenRedisFails(t *testing.T) {
	runID := uuid.New()
	store := &fakeStore{ids: []uuid.UUID{runID}}
	enqueuer := &fakeEnqueuer{err: errors.New("redis unavailable")}
	d := New(store, enqueuer, 0, 1, nil)
	if err := d.DispatchPending(context.Background()); err == nil {
		t.Fatal("expected enqueue error")
	}
	if len(store.marked) != 0 {
		t.Fatalf("run was marked enqueued after failure: %#v", store.marked)
	}
}

func TestDispatchPendingTreatsTaskIDConflictAsDelivered(t *testing.T) {
	runID := uuid.New()
	store := &fakeStore{ids: []uuid.UUID{runID}}
	enqueuer := &fakeEnqueuer{err: asynq.ErrTaskIDConflict}
	d := New(store, enqueuer, 0, 1, nil)
	if err := d.DispatchPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.marked) != 1 {
		t.Fatalf("conflict was not marked delivered: %#v", store.marked)
	}
}
