package check

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

const TypeCheckRun = "check:run"

type CheckRunPayload struct {
	RunID uuid.UUID `json:"run_id"`
}

func NewCheckRunTask(runID uuid.UUID) (*asynq.Task, error) {
	if runID == uuid.Nil {
		return nil, errors.New("run_id must not be empty")
	}
	payload, err := json.Marshal(CheckRunPayload{RunID: runID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeCheckRun, payload), nil
}

func ParseCheckRunPayload(data []byte) (CheckRunPayload, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var payload CheckRunPayload
	if err := decoder.Decode(&payload); err != nil {
		return CheckRunPayload{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return CheckRunPayload{}, errors.New("payload must contain one JSON object")
	}
	if payload.RunID == uuid.Nil {
		return CheckRunPayload{}, errors.New("run_id must not be empty")
	}
	return payload, nil
}
