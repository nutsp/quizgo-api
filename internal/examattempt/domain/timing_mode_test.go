package domain

import "testing"

func TestNormalizeTimingMode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty defaults to countdown", in: "", want: TimingModeCountdown},
		{name: "countdown stays countdown", in: TimingModeCountdown, want: TimingModeCountdown},
		{name: "elapsed stays elapsed", in: TimingModeElapsed, want: TimingModeElapsed},
		{name: "invalid defaults to countdown", in: "freeform", want: TimingModeCountdown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeTimingMode(tt.in); got != tt.want {
				t.Fatalf("NormalizeTimingMode(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestExamAttemptUsesCountdownDeadlineOnlyForCountdownMode(t *testing.T) {
	if !usesCountdownDeadline(TimingModeCountdown) {
		t.Fatal("countdown mode should use countdown deadline")
	}
	if usesCountdownDeadline(TimingModeElapsed) {
		t.Fatal("elapsed mode should not use countdown deadline")
	}
	if !usesCountdownDeadline("") {
		t.Fatal("empty timing mode should default to countdown deadline")
	}
}
