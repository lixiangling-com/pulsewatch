package check

type Outcome string

const (
	OutcomeSuccess       Outcome = "success"
	OutcomeTargetFailure Outcome = "target_failure"
	OutcomeBlocked       Outcome = "blocked"
	OutcomeSystemFailure Outcome = "system_failure"
)

type Transition struct {
	State     string
	RunStatus string
}

// NextState advances only on completed target checks. Safety blocks and system
// failures are recorded separately and must not influence the monitor state.
func NextState(current string, outcome Outcome) Transition {
	if current == "paused" || outcome == OutcomeBlocked || outcome == OutcomeSystemFailure {
		return Transition{State: current}
	}
	if outcome == OutcomeSuccess {
		switch current {
		case "pending", "up", "confirming_up":
			return Transition{State: "up", RunStatus: "succeeded"}
		case "confirming_down":
			return Transition{State: "up", RunStatus: "succeeded"}
		case "down":
			return Transition{State: "confirming_up", RunStatus: "succeeded"}
		}
	}
	if outcome == OutcomeTargetFailure {
		switch current {
		case "pending", "up":
			return Transition{State: "confirming_down", RunStatus: "failed"}
		case "confirming_down":
			return Transition{State: "down", RunStatus: "failed"}
		case "down", "confirming_up":
			return Transition{State: "down", RunStatus: "failed"}
		}
	}
	return Transition{State: current}
}
