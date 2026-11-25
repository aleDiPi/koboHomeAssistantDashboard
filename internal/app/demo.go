package app

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/config"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/ha"
)

// Demo is a fake Home Assistant, to try the interface without a token.
type Demo struct {
	mu      sync.Mutex
	states  map[string]ha.State
	updates chan struct{}
	layout  config.Layout
}

func NewDemo(c config.Config) *Demo {
	d := &Demo{states: map[string]ha.State{}, updates: make(chan struct{}, 1), layout: c.LocalLayout()}
	for _, it := range c.Sensors {
		d.states[it.Entity] = ha.State{State: "19.4", Attr: map[string]any{"unit_of_measurement": "°C"}}
	}
	for i, it := range c.Lights {
		state := "off"
		if i%3 == 0 {
			state = "on"
		}
		d.states[it.Entity] = ha.State{State: state, Attr: map[string]any{}}
	}
	for i, it := range c.Actions {
		state, attr := "off", map[string]any{}
		if i == 0 {
			state = "on"
			attr["last_triggered"] = time.Now().Add(-150 * time.Second).Format(time.RFC3339)
		}
		d.states[it.Entity] = ha.State{State: state, Attr: attr}
		if strings.HasPrefix(it.Timer, "input_number.") {
			d.states[it.Timer] = ha.State{State: "10.0", Attr: map[string]any{}}
		}
	}
	for i, it := range c.Windows {
		state := "off"
		if strings.HasPrefix(it.Entity, "cover.") {
			state = "closed"
		}
		if i == 0 {
			state = map[string]string{"off": "on", "closed": "open"}[state]
		}
		d.states[it.Entity] = ha.State{State: state, Attr: map[string]any{"device_class": "window"}}
	}
	for _, it := range c.Notifications {
		d.states[it.Entity] = ha.State{State: "on", Attr: map[string]any{}}
	}
	for i, it := range c.Climate {
		state := "off"
		if i == 0 {
			state = "cool"
		}
		d.states[it.Entity] = ha.State{State: state, Attr: map[string]any{
			"temperature": 22.0, "current_temperature": 19.0, "min_temp": 16.0, "max_temp": 30.0,
			"hvac_modes": []any{"off", "cool", "dry", "fan_only", "auto", "heat"},
		}}
	}
	return d
}

func (d *Demo) Snapshot() (map[string]ha.State, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make(map[string]ha.State, len(d.states))
	for k, v := range d.states {
		out[k] = v
	}
	return out, true, nil
}

func (d *Demo) Updates() <-chan struct{} { return d.updates }

// Layout is the one of the configuration file.
func (d *Demo) Layout() config.Layout { return d.layout }

// Events invents a few appointments.
func (d *Demo) Events() []ha.Event {
	now := time.Now().Truncate(time.Hour)
	event := func(after time.Duration, summary string) ha.Event {
		var calendar string
		if ids := d.layout.CalendarIDs(); len(ids) > 0 {
			calendar = ids[0]
		}
		return ha.Event{Calendar: calendar, Summary: summary, Start: now.Add(after), End: now.Add(after + time.Hour)}
	}
	return []ha.Event{
		event(0, "Riscaldamento"),
		event(5*time.Hour, "Cena con amici"),
		event(26*time.Hour, "Dentista"),
		event(100*time.Hour, "Raccolta carta"),
		event(200*time.Hour, "Compleanno"),
	}
}

// History invents a daily temperature curve for each sensor.
func (d *Demo) History() map[string][]ha.Point {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := map[string][]ha.Point{}
	now := time.Now()
	for id, st := range d.states {
		if !strings.HasPrefix(id, "sensor.") {
			continue
		}
		base, _ := strconv.ParseFloat(st.State, 64)
		for h := 24; h > 0; h-- {
			v := base + 1.5*math.Sin(float64(h)/4)
			out[id] = append(out[id], ha.Point{Time: now.Add(-time.Duration(h) * time.Hour), Value: v})
		}
	}
	return out
}

func (d *Demo) Call(_, service, entity string, data map[string]any) error {
	d.mu.Lock()
	st := d.states[entity]
	attr := map[string]any{}
	for k, v := range st.Attr {
		attr[k] = v
	}
	st.Attr = attr
	domain, _, _ := strings.Cut(entity, ".")
	switch service {
	case "toggle":
		if st.State == "on" {
			st.State = "off"
		} else {
			st.State = "on"
		}
	case "open_cover":
		st.State = "open"
	case "close_cover":
		st.State = "closed"
	case "turn_off":
		st.State = "off"
	case "turn_on":
		st.State = "on"
		if domain == "script" {
			st.Attr["last_triggered"] = time.Now().Format(time.RFC3339)
		}
		if domain == "climate" {
			modes := st.Strings("hvac_modes")
			if i := slices.IndexFunc(modes, func(m string) bool { return m != "off" }); i >= 0 {
				st.State = modes[i]
			}
		}
	case "set_hvac_mode":
		st.State, _ = data["hvac_mode"].(string)
	case "set_temperature":
		st.Attr["temperature"] = data["temperature"]
	}
	d.states[entity] = st
	d.mu.Unlock()
	select {
	case d.updates <- struct{}{}:
	default:
	}
	return nil
}
