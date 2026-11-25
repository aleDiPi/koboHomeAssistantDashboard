package app

import (
	"slices"
	"strings"
	"sync"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/config"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/ha"
)

// Demo is a fake Home Assistant, to try the interface without a token.
type Demo struct {
	mu      sync.Mutex
	states  map[string]ha.State
	updates chan struct{}
}

func NewDemo(c config.Config) *Demo {
	d := &Demo{states: map[string]ha.State{}, updates: make(chan struct{}, 1)}
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
	case "turn_off":
		st.State = "off"
	case "turn_on":
		st.State = "on"
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
