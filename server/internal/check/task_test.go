package check

import (
	"testing"

	"github.com/google/uuid"
)

func TestCheckRunTaskRoundTrip(t *testing.T) {
	runID := uuid.New()
	task, err := NewCheckRunTask(runID)
	if err != nil {
		t.Fatal(err)
	}
	if task.Type() != TypeCheckRun {
		t.Fatalf("type = %q", task.Type())
	}
	payload, err := ParseCheckRunPayload(task.Payload())
	if err != nil || payload.RunID != runID {
		t.Fatalf("payload = %#v, err = %v", payload, err)
	}
}

func TestCheckRunPayloadRejectsInvalidInput(t *testing.T) {
	for name, input := range map[string][]byte{
		"missing":       []byte(`{}`),
		"invalid uuid":  []byte(`{"run_id":"nope"}`),
		"unknown field": []byte(`{"run_id":"00000000-0000-0000-0000-000000000001","url":"https://example.com"}`),
		"extra json":    []byte(`{"run_id":"00000000-0000-0000-0000-000000000001"}{}`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseCheckRunPayload(input); err == nil {
				t.Fatal("expected payload error")
			}
		})
	}
	if _, err := NewCheckRunTask(uuid.Nil); err == nil {
		t.Fatal("expected empty run ID error")
	}
}
