package app

import (
	"errors"
	"testing"
	"time"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/config"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/ha"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/ui"
)

type fakePower struct {
	plugged bool
	naps    []time.Duration
	err     error
}

func (f *fakePower) Plugged() bool { return f.plugged }
func (f *fakePower) Suspend(d time.Duration) error {
	f.naps = append(f.naps, d)
	return f.err
}

func TestNapDependsOnPresenceAndPower(t *testing.T) {
	c := config.Default().Power
	c.Standby, c.Presence = true, []string{"person.a", "person.b"}
	states := func(a, b string) map[string]ha.State {
		return map[string]ha.State{"person.a": {State: a}, "person.b": {State: b}}
	}
	for name, tc := range map[string]struct {
		states  map[string]ha.State
		plugged bool
		want    time.Duration
	}{
		"home on power":   {states("home", "not_home"), true, 0},
		"home on battery": {states("not_home", "home"), false, c.HomeRefresh},
		"away on battery": {states("not_home", "Work"), false, c.AwayRefresh},
		"away on power":   {states("not_home", "not_home"), true, c.AwayRefresh},
		"unknown people":  {nil, true, 0},
	} {
		if got := nap(c, tc.states, tc.plugged); got != tc.want {
			t.Errorf("%s: %v, want %v", name, got, tc.want)
		}
	}
	c.Standby = false
	if nap(c, states("not_home", "not_home"), false) != 0 {
		t.Error("standby disabled")
	}
}

func TestSleepsOnlyWhenIdle(t *testing.T) {
	a, _, _ := testApp(ui.Lights)
	a.cfg.Power.Standby = true
	power := &fakePower{}
	a.power = power
	now := time.Now()

	a.lastTouch = now
	a.standby(now)
	if len(power.naps) != 0 {
		t.Fatal("slept while in use")
	}
	a.standby(now.Add(a.cfg.Power.Idle + time.Second))
	if len(power.naps) != 1 || power.naps[0] != a.cfg.Power.HomeRefresh {
		t.Fatalf("naps %v", power.naps)
	}
	// The fake returns at once: that is a wake by the button, so stay awake.
	a.standby(time.Now())
	a.standby(time.Now().Add(glance + time.Second))
	if len(power.naps) != 1 {
		t.Fatalf("went straight back to sleep: %v", power.naps)
	}
	if a.view != ui.Home || a.asleep {
		t.Error("should show the home page, awake")
	}
}

func TestFailedSuspendIsNotRetriedAtOnce(t *testing.T) {
	a, _, _ := testApp(ui.Home)
	a.cfg.Power.Standby = true
	power := &fakePower{err: errors.New("no")}
	a.power = power
	now := time.Now().Add(time.Hour)
	a.standby(now)
	a.standby(now.Add(time.Second))
	if len(power.naps) != 1 {
		t.Fatalf("naps %v", power.naps)
	}
}
