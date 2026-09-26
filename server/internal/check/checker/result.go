package checker

import (
	"context"
	"time"
)

type Outcome string

const (
	OutcomeSuccess       Outcome = "success"
	OutcomeTargetFailure Outcome = "target_failure"
	OutcomeSystemFailure Outcome = "system_failure"
	OutcomeBlocked       Outcome = "blocked"
)

type Target struct {
	URL            string
	ExpectedStatus int
}

type Result struct {
	Outcome    Outcome
	ErrorCode  string
	Summary    string
	StatusCode int
	Latency    time.Duration
}

type Checker interface {
	Check(context.Context, Target) Result
}
