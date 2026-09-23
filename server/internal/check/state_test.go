package check

import "testing"

func TestNextState(t *testing.T) {
	tests := []struct {
		from    string
		outcome Outcome
		want    string
		run     string
	}{
		{"pending", OutcomeSuccess, "up", "succeeded"},
		{"pending", OutcomeTargetFailure, "confirming_down", "failed"},
		{"up", OutcomeTargetFailure, "confirming_down", "failed"},
		{"confirming_down", OutcomeTargetFailure, "down", "failed"},
		{"confirming_down", OutcomeSuccess, "up", "succeeded"},
		{"down", OutcomeSuccess, "confirming_up", "succeeded"},
		{"confirming_up", OutcomeSuccess, "up", "succeeded"},
		{"confirming_up", OutcomeTargetFailure, "down", "failed"},
		{"up", OutcomeBlocked, "up", ""},
		{"down", OutcomeSystemFailure, "down", ""},
		{"paused", OutcomeSuccess, "paused", ""},
	}
	for _, tt := range tests {
		t.Run(tt.from+"/"+string(tt.outcome), func(t *testing.T) {
			got := NextState(tt.from, tt.outcome)
			if got.State != tt.want || got.RunStatus != tt.run {
				t.Fatalf("NextState()=%+v, want state=%s run=%s", got, tt.want, tt.run)
			}
		})
	}
}
