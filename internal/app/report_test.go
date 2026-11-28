package app

import (
	"testing"
	"time"
)

func TestReportingIsRareOnBattery(t *testing.T) {
	t0 := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	on := deviceState{battery: 80, plugged: true}
	off := deviceState{battery: 80}
	lower := deviceState{battery: 79}
	every := 15 * time.Minute
	for name, tc := range map[string]struct {
		after         time.Duration
		current, sent deviceState
		want          bool
	}{
		"unchanged on power": {30 * time.Second, on, on, false},
		"a minute on power":  {time.Minute, on, on, true},
		"change on power":    {time.Second, deviceState{battery: 81, plugged: true}, on, true},
		"cable removed":      {time.Second, off, on, true},
		"change on battery":  {time.Minute, lower, off, false},
		"rarely on battery":  {every, lower, off, true},
		"cable plugged back": {time.Second, on, off, true},
	} {
		if got := due(t0.Add(tc.after), t0, tc.current, tc.sent, every); got != tc.want {
			t.Errorf("%s: %v", name, got)
		}
	}
	if due(t0.Add(time.Hour), t0, lower, off, 0) {
		t.Error("0 must mean never on battery")
	}
}
