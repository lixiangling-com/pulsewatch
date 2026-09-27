package notification

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

type fakeStore struct {
	ids      []uuid.UUID
	prepared []uuid.UUID
	queued   []uuid.UUID
}

func (s *fakeStore) PendingIDs(context.Context, int) ([]uuid.UUID, error) { return s.ids, nil }
func (s *fakeStore) PrepareRetry(_ context.Context, id uuid.UUID) error {
	s.prepared = append(s.prepared, id)
	return nil
}
func (s *fakeStore) MarkQueued(_ context.Context, id uuid.UUID) error {
	s.queued = append(s.queued, id)
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

func TestDispatcherLeavesNotificationUnqueuedWhenEnqueueFails(t *testing.T) {
	id := uuid.New()
	store := &fakeStore{ids: []uuid.UUID{id}}
	client := &fakeEnqueuer{err: context.DeadlineExceeded}
	d := NewDispatcher(store, client, 0, 10, nil)
	if err := d.DispatchPending(context.Background()); err == nil {
		t.Fatal("expected enqueue error")
	}
	if len(store.prepared) != 1 || store.prepared[0] != id {
		t.Fatalf("prepared = %#v, want notification %s", store.prepared, id)
	}
	if len(store.queued) != 0 {
		t.Fatalf("queued = %#v, want no notifications marked queued", store.queued)
	}
}

func TestDispatcherUsesNotificationIDTaskID(t *testing.T) {
	id := uuid.New()
	store := &fakeStore{ids: []uuid.UUID{id}}
	client := &fakeEnqueuer{}
	d := NewDispatcher(store, client, 0, 10, nil)
	if err := d.DispatchPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.task == nil || client.task.Type() != TypeMailSend {
		t.Fatalf("task = %#v", client.task)
	}
	if len(client.options) != 1 || client.options[0].Type() != asynq.TaskIDOpt || client.options[0].Value() != id.String() {
		t.Fatalf("options = %#v", client.options)
	}
	if len(store.queued) != 1 || store.queued[0] != id {
		t.Fatalf("queued = %#v", store.queued)
	}
}
