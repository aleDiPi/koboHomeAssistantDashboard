package config

import (
	"strconv"
	"strings"
)

// Page is one screen of the dashboard, reached from the bottom bar.
type Page struct {
	Title     string
	Sensors   []Item // value and 24 h graph
	Lights    []Item // anything that is switched with one touch
	Climate   []Item
	Windows   []Item // contact sensors and covers
	Actions   []Item // scripts, scenes and buttons
	Calendars []Item
}

// Layout is what the screens show. It comes from a Home Assistant dashboard
// or, failing that, from the lists in the configuration file.
type Layout struct {
	Pages         []Page
	Notifications []Item // chips in the header while on or open
}

// LocalLayout builds the pages from the lists in the configuration file.
func (c Config) LocalLayout() Layout {
	return Layout{
		Notifications: c.Notifications,
		Pages: []Page{
			{Title: "Casa", Sensors: c.Sensors, Calendars: c.Calendars},
			{Title: "Luci", Lights: c.Lights},
			{Title: "Clima", Climate: c.Climate, Windows: c.Windows, Actions: c.Actions},
		},
	}
}

func unique(lists ...[]Item) []string {
	seen := map[string]bool{}
	var out []string
	for _, list := range lists {
		for _, it := range list {
			if !seen[it.Entity] {
				seen[it.Entity] = true
				out = append(out, it.Entity)
			}
		}
	}
	return out
}

func (l Layout) collect(pick func(Page) []Item) []string {
	var lists [][]Item
	for _, p := range l.Pages {
		lists = append(lists, pick(p))
	}
	return unique(lists...)
}

// Graphed lists the sensors that need their history.
func (l Layout) Graphed() []string { return l.collect(func(p Page) []Item { return p.Sensors }) }

// CalendarIDs lists the calendars that need their events.
func (l Layout) CalendarIDs() []string { return l.collect(func(p Page) []Item { return p.Calendars }) }

// Watched lists every entity whose state is needed: those on screen, those
// behind the notifications and those that drive the screen light.
func (c Config) Watched(l Layout) []string {
	lists := [][]Item{l.Notifications}
	for _, p := range l.Pages {
		lists = append(lists, p.Sensors, p.Lights, p.Climate, p.Windows, p.Actions, p.Calendars)
		var timers []Item
		for _, a := range p.Actions {
			// A timer is either a number of minutes or an entity.
			if _, err := strconv.ParseFloat(a.Timer, 64); err != nil && strings.Contains(a.Timer, ".") {
				timers = append(timers, Item{Entity: a.Timer})
			}
		}
		lists = append(lists, timers)
	}
	if c.Power.Standby {
		for _, id := range c.Power.Presence {
			lists = append(lists, []Item{{Entity: id}})
		}
	}
	if c.Frontlight.Manage {
		var light []Item
		for _, id := range append([]string{c.Frontlight.Sun}, c.Frontlight.NightWhenOff...) {
			if id != "" {
				light = append(light, Item{Entity: id})
			}
		}
		lists = append(lists, light)
	}
	return unique(lists...)
}
