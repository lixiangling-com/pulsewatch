package notification

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const TypeMailSend = "mail:send"

type MailPayload struct {
	NotificationID uuid.UUID `json:"notification_id"`
}

func NewMailTask(id uuid.UUID) (*asynq.Task, error) {
	if id == uuid.Nil {
		return nil, errors.New("notification_id must not be empty")
	}
	body, err := json.Marshal(MailPayload{NotificationID: id})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeMailSend, body), nil
}

func ParseMailPayload(data []byte) (MailPayload, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var payload MailPayload
	if err := decoder.Decode(&payload); err != nil {
		return MailPayload{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return MailPayload{}, errors.New("payload must contain one JSON object")
	}
	if payload.NotificationID == uuid.Nil {
		return MailPayload{}, errors.New("notification_id must not be empty")
	}
	return payload, nil
}
