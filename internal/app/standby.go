package app

import (
	"log"
	"time"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/config"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/ha"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/kobo"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/ui"
)

// Power is the part of the hardware that standby needs.
type Power interface {
	Plugged() bool
	// Suspend returns when the device is awake again.
	Suspend(time.Duration) error
}

type device struct{}

func (device) Plugged() bool                 { return kobo.Plugged() }
func (device) Suspend(d time.Duration) error { return kobo.Suspend(d) }

const (
	// After a timed wake: how long to wait for Home Assistant, and then for
	// the screen to show the news, before sleeping again.
	connectLimit = 45 * time.Second
	glance       = 8 * time.Second
	// Never sleep right after starting, so that there is time to step in.
	startGrace = 2 * time.Minute
	// After a failed suspend, leave the device awake for a while.
	retryAfter = 10 * time.Minute
)

// nap decides how long to sleep between two refreshes; zero means stay awake.
// At home on external power nothing sleeps; at home on battery it naps; away
// from home it naps longer, whatever the power.
func nap(c config.Power, states map[string]ha.State, plugged bool) time.Duration {
	if !c.Standby {
		return 0
	}
	away := len(c.Presence) > 0
	for _, id := range c.Presence {
		// Unknown counts as at home.
		switch states[id].State {
		case "home", "", "unknown", "unavailable":
			away = false
		}
	}
	switch {
	case away:
		return c.AwayRefresh
	case plugged:
		return 0
	}
	return c.HomeRefresh
}

// powerSave follows the cable: on battery processor and Wi-Fi rest.
func (a *App) powerSave(now time.Time) {
	if !a.cfg.Power.Saving || now.Sub(a.savingChecked) < reportCheck {
		return
	}
	a.savingChecked = now
	saving := !a.plugged()
	if a.saving != nil && *a.saving == saving {
		return
	}
	a.saving = &saving
	log.Printf("power saving %v: %s", saving, kobo.PowerSave(saving))
}

// standby sleeps when it is time to. It is called regularly.
func (a *App) standby(now time.Time) {
	if a.power == nil || now.Before(a.awakeUntil) || len(a.temps) > 0 || a.armed.Kind != ui.None {
		return
	}
	states, connected, _ := a.src.Snapshot()
	if a.waking {
		if (!connected || len(states) == 0) && now.Before(a.wakeLimit) {
			return
		}
		// Fresh data (or none to be had): show it, and give it time to land.
		a.waking = false
		a.render(false)
		a.awakeUntil = now.Add(glance)
		return
	}
	d := nap(a.cfg.Power, states, a.power.Plugged())
	if d <= 0 || now.Sub(a.lastTouch) < a.cfg.Power.Idle {
		a.asleep = false
		return
	}
	a.sleep(now, d)
}

func (a *App) sleep(asleepAt time.Time, d time.Duration) {
	// The screen keeps showing this while the device sleeps.
	a.asleep, a.view, a.page = true, ui.Home, 0
	a.render(false)
	if a.cfg.Frontlight.Manage {
		a.light = 0
		if err := kobo.SetFrontlight(0); err != nil {
			log.Printf("frontlight: %v", err)
		}
	}

	log.Printf("standby for %v, battery %d%%", d, kobo.Battery())
	// The wall clock keeps counting while asleep, the monotonic one does not.
	start := time.Now().Round(0)
	err := a.power.Suspend(d)
	slept := time.Now().Round(0).Sub(start)
	now := asleepAt.Add(slept)
	a.src.Reconnect()
	a.saving = nil
	if err != nil {
		log.Printf("standby failed: %v", err)
		a.asleep, a.awakeUntil = false, now.Add(retryAfter)
		return
	}
	log.Printf("awake after %v", slept.Round(time.Second))
	a.waking, a.wakeLimit = true, now.Add(connectLimit)
	// Waking well before the alarm means somebody pressed the button.
	if slept < d-10*time.Second {
		a.userWake(now)
	}
}

// userWake keeps the device awake as after a touch.
func (a *App) userWake(now time.Time) {
	a.lastTouch, a.asleep = now, false
	a.updateLight(now)
}
