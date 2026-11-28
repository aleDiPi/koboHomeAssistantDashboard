package app

import (
	"log"
	"time"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/config"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/ha"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/kobo"
)

// lightLevel decides the screen light: off by day; after sunset a boost
// following a touch, otherwise a faint light only when the room is dark.
func lightLevel(c config.Frontlight, states map[string]ha.State, sinceTouch time.Duration) int {
	if states[c.Sun].State != "below_horizon" {
		return 0
	}
	if sinceTouch < c.TouchTime {
		return c.TouchLevel
	}
	for _, id := range c.NightWhenOff {
		if states[id].State == "on" {
			return 0
		}
	}
	return c.NightLevel
}

func (a *App) updateLight(now time.Time) {
	// Nobody is looking during a timed wake: leave the light off.
	if !a.cfg.Frontlight.Manage || a.dark {
		return
	}
	states, connected, _ := a.src.Snapshot()
	if !connected {
		return // keep the current level while Home Assistant is unreachable
	}
	level := lightLevel(a.cfg.Frontlight, states, now.Sub(a.lastTouch))
	if level == a.light {
		return
	}
	a.light = level
	if err := kobo.SetFrontlight(level); err != nil {
		log.Printf("frontlight: %v", err)
	}
}
