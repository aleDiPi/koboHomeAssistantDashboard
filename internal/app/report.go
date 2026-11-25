package app

import (
	"log"
	"strconv"
	"time"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/ha"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/kobo"
)

const (
	reportOnPower = time.Minute      // on mains power
	reportCheck   = 10 * time.Second // how often changes are looked for
)

// deviceState is what is published about the Kobo itself.
type deviceState struct {
	battery int
	voltage float64 // volts, 0 if unknown
	plugged bool
	light   int // -1 if the light is not managed
	asleep  bool
}

// due decides whether to publish now. On mains power: on every change and once
// a minute. On battery, to save it: only rarely, plus once when the cable is
// removed, so that Home Assistant knows.
func due(now, last time.Time, current, sent deviceState, onBattery time.Duration) bool {
	// The voltage jitters all the time: it travels with the other reasons.
	current.voltage, sent.voltage = 0, 0
	switch {
	case last.IsZero() || current.plugged != sent.plugged:
		return true
	case current.plugged:
		return current != sent || now.Sub(last) >= reportOnPower
	}
	return onBattery > 0 && now.Sub(last) >= onBattery
}

func sensors(s deviceState) []ha.Sensor {
	attr := func(name string, extra ...string) map[string]any {
		m := map[string]any{"friendly_name": name}
		for i := 0; i+1 < len(extra); i += 2 {
			m[extra[i]] = extra[i+1]
		}
		return m
	}
	plugged, mode := "off", "attivo"
	if s.plugged {
		plugged = "on"
	}
	if s.asleep {
		mode = "standby"
	}
	out := []ha.Sensor{
		{Entity: "sensor.kobo_batteria", State: strconv.Itoa(s.battery),
			Attr: attr("Kobo batteria", "unit_of_measurement", "%", "device_class", "battery", "state_class", "measurement")},
		{Entity: "binary_sensor.kobo_alimentazione", State: plugged,
			Attr: attr("Kobo alimentazione", "device_class", "plug")},
		{Entity: "sensor.kobo_modalita", State: mode, Attr: attr("Kobo modalità", "icon", "mdi:sleep")},
	}
	// The Glo reports a percentage where the voltage should be: publish only
	// what can be the voltage of a lithium cell.
	if s.voltage >= 2.5 && s.voltage <= 5 {
		out = append(out, ha.Sensor{Entity: "sensor.kobo_tensione_batteria", State: strconv.FormatFloat(s.voltage, 'f', 2, 64),
			Attr: attr("Kobo tensione batteria", "unit_of_measurement", "V", "device_class", "voltage", "state_class", "measurement")})
	}
	if s.light >= 0 {
		out = append(out, ha.Sensor{Entity: "sensor.kobo_luce", State: strconv.Itoa(s.light),
			Attr: attr("Kobo luce schermo", "unit_of_measurement", "%", "icon", "mdi:brightness-6")})
	}
	return out
}

// report publishes the device sensors when due. It does not block.
func (a *App) report(now time.Time) {
	if !a.cfg.Power.Report || a.waking || now.Sub(a.reportChecked) < reportCheck {
		return
	}
	a.reportChecked = now
	_, connected, _ := a.src.Snapshot()
	battery := a.batteryLevel()
	if !connected || battery < 0 { // no battery: not running on the device
		return
	}
	current := deviceState{battery: battery, voltage: kobo.Voltage(), plugged: a.plugged(), light: a.light, asleep: a.asleep}
	if !a.cfg.Frontlight.Manage {
		current.light = -1
	}
	if !due(now, a.reported, current, a.sent, a.cfg.Power.ReportOnBattery) {
		return
	}
	a.reported, a.sent = now, current
	go func() {
		if err := a.src.Report(sensors(current)); err != nil {
			log.Printf("report: %v", err)
		}
	}()
}
