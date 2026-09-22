package consumer

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	checktask "github.com/lixiangling-com/pulsewatch/server/internal/check"
)

type fakeProcessor struct {
	ids []uuid.UUID
	err error
}

func (p *fakeProcessor) Process(_ context.Context, id uuid.UUID) error {
	p.ids = append(p.ids, id)
	return p.err
}

func TestHandleCheckRunPassesRunIDToProcessor(t *testing.T) {
	id := uuid.New()
	task, err := checktask.NewCheckRunTask(id)
	if err != nil {
		t.Fatal(err)
	}
	processor := &fakeProcessor{}
	c := New(processor, nil)
	if err := c.HandleCheckRun(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if len(processor.ids) != 1 || processor.ids[0] != id {
		t.Fatalf("processed = %#v", processor.ids)
	}
}

func TestHandleCheckRunRejectsMalformedPayload(t *testing.T) {
	c := New(&fakeProcessor{}, nil)
	if err := c.HandleCheckRun(context.Background(), asynq.NewTask(checktask.TypeCheckRun, []byte(`{"run_id":"bad","extra":true}`))); err == nil {
		t.Fatal("expected payload error")
	}
}
