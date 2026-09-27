package notification

import (
	"testing"

	"github.com/google/uuid"
)

func TestMailPayloadStrict(t *testing.T) {
	id := uuid.New()
	task, err := NewMailTask(id)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := ParseMailPayload(task.Payload())
	if err != nil || payload.NotificationID != id {
		t.Fatalf("payload = %#v, err = %v", payload, err)
	}
	if _, err := ParseMailPayload([]byte(`{"notification_id":"` + id.String() + `","extra":true}`)); err == nil {
		t.Fatal("expected unknown field rejection")
	}
}
