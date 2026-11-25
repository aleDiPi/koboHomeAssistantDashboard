// Package app connects Home Assistant, the screen and the touch panel.
package app

import (
	"bytes"
	"context"
	"errors"
	"image"
	"log"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/config"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/ha"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/kobo"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/screen"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/ui"
)

// Source provides entity states and runs services.
type Source interface {
	Snapshot() (map[string]ha.State, bool, error)
	Updates() <-chan struct{}
	History() map[string][]ha.Point
	Events() []ha.Event
	Layout() config.Layout
	Call(domain, service, entity string, data map[string]any) error
	// Reconnect starts afresh after the device has slept.
	Reconnect()
	// Report publishes the sensors of the device.
	Report([]ha.Sensor) error
}

const (
	settleDelay   = 150 * time.Millisecond // groups bursts of state changes
	pressTime     = 600 * time.Millisecond // how long a tapped tile stays inverted
	tempDelay     = 900 * time.Millisecond // idle time before sending a temperature
	tempExpiry    = 8 * time.Second        // give up waiting for confirmation
	confirmWindow = 4 * time.Second
	maxPartials   = 60 // flash after this many partial updates, against ghosting
)

type pending struct {
	value         float64
	sendAt, until time.Time
	sent          bool
}

type App struct {
	cfg  config.Config
	src  Source
	disp screen.Display
	loc  *time.Location

	frame      ui.Frame
	view       ui.View
	page       int
	temps      map[string]*pending
	armed      ui.Action // waiting for a second touch
	armedUntil time.Time
	lastFlash  time.Time
	partials   int

	lastTouch  time.Time
	rendered   time.Time
	power      Power // nil when standby is not possible
	asleep     bool  // showing the standby notice
	waking     bool  // waiting for fresh data after a suspend
	wakeLimit  time.Time
	awakeUntil time.Time
	buttons    <-chan struct{}

	batteryLevel  func() int
	plugged       func() bool
	reported      time.Time
	reportChecked time.Time
	saving        *bool // nil until applied
	savingChecked time.Time
	wifiLost      bool // Wi-Fi did not come back from a suspend
	retryWifi     time.Time
	picture       *image.Gray // of the standby screen, nil if none
	pictures      []string
	picturePath   string
	sent          deviceState
	light         int // frontlight level in use, -1 before the first one
	battery       int
	batteryLog    time.Time
}

func New(cfg config.Config, src Source, disp screen.Display, loc *time.Location) *App {
	return &App{cfg: cfg, src: src, disp: disp, loc: loc, temps: map[string]*pending{}, light: -1, battery: -1,
		batteryLevel: kobo.Battery, plugged: kobo.Plugged}
}

// EnableStandby lets the device sleep; button reports presses of the power
// button while it is awake.
func (a *App) EnableStandby(button <-chan struct{}) {
	a.power, a.buttons = device{}, button
}

// Run serves taps and updates until the user taps "Esci" twice or ctx ends.
func (a *App) Run(ctx context.Context, taps <-chan image.Point) error {
	for _, line := range kobo.PowerInfo() {
		log.Printf("power supply: %s", line)
	}
	a.battery = kobo.Battery()
	a.lastTouch = time.Now()
	a.awakeUntil = a.lastTouch.Add(startGrace)
	a.updateLight(time.Time{})
	a.render(true)
	redraw := time.NewTimer(time.Hour)
	var redrawAt time.Time
	schedule := func(d time.Duration) {
		at := time.Now().Add(d)
		if redrawAt.IsZero() || at.Before(redrawAt) {
			redrawAt = at
			redraw.Reset(d)
		}
	}
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	minute := a.now().Minute()

	for {
		select {
		case <-ctx.Done():
			log.Printf("closing: %v", context.Cause(ctx))
			return ctx.Err()
		case <-a.src.Updates():
			schedule(settleDelay)
			a.updateLight(time.Now())
		case <-redraw.C:
			redrawAt = time.Time{}
			// Just woken, the screen keeps the old picture until there is news.
			if !a.waking {
				a.render(false)
			}
		case <-a.buttons:
			log.Print("power button")
			a.userWake(time.Now())
			// A flash answers the press even if nothing else changed.
			a.render(true)
		case p, ok := <-taps:
			if !ok {
				return errors.New("touch input lost")
			}
			a.userWake(time.Now())
			log.Printf("tap %v", p)
			exit, delay := a.tap(p)
			if exit {
				log.Print("closed with Esci")
				return nil
			}
			if delay >= 0 {
				schedule(delay)
			}
		case <-tick.C:
			now := time.Now()
			a.updateLight(now)
			if now.Sub(a.batteryLog) >= 10*time.Minute {
				// A record of the drain, to size the power supply.
				a.batteryLog = now
				if level := kobo.Battery(); level >= 0 {
					log.Printf("battery %d%% %.2fV %s", level, kobo.Voltage(), kobo.BatteryStatus())
				}
			}
			if a.returnHome(now) {
				schedule(0)
			}
			if a.frame.Countdown && now.Sub(a.rendered) >= ui.CountdownStep {
				schedule(0)
			}
			a.powerSave(now)
			a.report(now)
			a.standby(now)
			if a.flushTemps(now) {
				schedule(0)
			}
			if a.armed.Kind != ui.None && now.After(a.armedUntil) {
				a.armed = ui.Action{}
				schedule(0)
			}
			if m := a.now().Minute(); m != minute {
				minute = m
				a.battery = kobo.Battery()
				schedule(0)
			}
			if a.cfg.FullRefreshEvery > 0 && now.Sub(a.lastFlash) >= a.cfg.FullRefreshEvery {
				a.render(true)
			}
		}
	}
}

func (a *App) now() time.Time { return time.Now().In(a.loc) }

func (a *App) model() ui.Model {
	states, connected, err := a.src.Snapshot()
	m := ui.Model{
		Layout: a.src.Layout(), States: states,
		Config: a.cfg.Screensaver, Picture: a.picture, Connected: connected,
		Now: a.now(), View: a.view, Page: a.page, Pending: map[string]float64{},
		Armed: a.armed,

		History: a.src.History(), Battery: a.battery, Events: a.src.Events(),
	}
	switch {
	case errors.Is(err, ha.ErrAuth):
		m.Status = "Token non valido"
	case err != nil:
		m.Status = "Non connesso"
	default:
		m.Status = "Connessione…"
	}
	if a.wifiLost {
		m.Connected, m.Status = false, "Wi-Fi perso · Esci per riavviare"
	}
	for id, p := range a.temps {
		if v, ok := states[id].Float("temperature"); ok && p.sent && math.Abs(v-p.value) < 0.01 {
			delete(a.temps, id)
			continue
		}
		m.Pending[id] = p.value
	}
	return m
}

// render draws the current model and refreshes only the areas that changed.
func (a *App) render(flash bool) {
	w, h := a.disp.Size()
	old := a.frame
	model := a.model()
	var next ui.Frame
	if a.asleep {
		next = ui.RenderSleep(model, w, h)
	} else {
		next = ui.Render(model, w, h)
	}
	a.rendered = time.Now()
	a.frame = next
	if next.View >= 0 {
		a.view, a.page = next.View, next.Page
	}

	var changed []image.Rectangle
	if old.Image == nil || old.View != next.View || old.Page != next.Page || a.partials >= maxPartials {
		flash = true
	}
	if !flash {
		for _, r := range next.Regions {
			if !same(old.Image, next.Image, r.Rect) {
				changed = append(changed, r.Rect)
			}
		}
		if len(changed) == 0 {
			return
		}
		// Many separate updates are slower than a single one.
		if len(changed) > 6 {
			union := changed[0]
			for _, r := range changed[1:] {
				union = union.Union(r)
			}
			changed = []image.Rectangle{union}
		}
	}

	if flash {
		a.lastFlash, a.partials = time.Now(), 0
		a.show(next.Image.Rect, screen.Flash)
		return
	}
	for _, r := range changed {
		a.partials++
		a.show(r, screen.Quality)
	}
}

func (a *App) show(r image.Rectangle, mode screen.Mode) {
	if err := a.disp.Show(a.frame.Image, r, mode); err != nil {
		log.Printf("screen: %v", err)
	}
}

func same(a, b *image.Gray, r image.Rectangle) bool {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		i, j := a.PixOffset(r.Min.X, y), a.PixOffset(r.Max.X, y)
		if !bytes.Equal(a.Pix[i:j], b.Pix[i:j]) {
			return false
		}
	}
	return true
}

// tap handles one touch. delay < 0 means no redraw is needed.
func (a *App) tap(p image.Point) (exit bool, delay time.Duration) {
	region, ok := a.frame.Hit(p)
	if !ok {
		return false, -1
	}
	// Immediate feedback: only the touched area, with the fast waveform. The
	// next render sees the difference and restores or updates it.
	ui.Invert(a.frame.Image, region)
	a.show(region.Rect, screen.Fast)
	a.partials++

	act := region.Action
	// Any other touch cancels a pending confirmation.
	confirmed := a.armed == act
	a.armed = ui.Action{}
	states, _, _ := a.src.Snapshot()
	st := states[act.Entity]
	domain, _, _ := strings.Cut(act.Entity, ".")

	switch act.Kind {
	case ui.Exit:
		if confirmed {
			return true, 0
		}
		a.arm(act)
		return false, 0
	case ui.OpenCover:
		if !confirmed {
			a.arm(act)
			return false, 0
		}
		a.call("cover", "open_cover", act.Entity, nil)
	case ui.CloseCover:
		a.call("cover", "close_cover", act.Entity, nil)
	case ui.Show:
		a.view, a.page = act.View, 0
		return false, 0
	case ui.NextPage:
		a.page = (a.frame.Page + 1) % a.frame.Pages
		return false, 0
	case ui.Toggle:
		a.call(domain, "toggle", act.Entity, nil)
	case ui.Run:
		// Scripts can do a lot at once (open windows): they ask twice.
		if domain == "script" && !confirmed {
			a.arm(act)
			return false, 0
		}
		service := "turn_on"
		if domain == "button" || domain == "input_button" {
			service = "press"
		}
		a.call(domain, service, act.Entity, nil)
	case ui.Power:
		service := "turn_on"
		if st.State != "off" {
			service = "turn_off"
		}
		a.call("climate", service, act.Entity, nil)
	case ui.Mode:
		if mode := nextMode(st.Strings("hvac_modes"), st.State); mode != "" {
			a.call("climate", "set_hvac_mode", act.Entity, map[string]any{"hvac_mode": mode})
		}
	case ui.TempDown, ui.TempUp:
		a.changeTemp(act, st)
		return false, pressTime / 2
	}
	return false, pressTime
}

func (a *App) call(domain, service, entity string, data map[string]any) {
	log.Printf("call %s.%s %s %v", domain, service, entity, data)
	if err := a.src.Call(domain, service, entity, data); err != nil {
		log.Printf("call failed: %v", err)
	}
}

func (a *App) arm(act ui.Action) {
	a.armed, a.armedUntil = act, time.Now().Add(confirmWindow)
}

// returnHome goes back to the home view after a while without touches.
func (a *App) returnHome(now time.Time) bool {
	if a.cfg.ReturnHome <= 0 || a.view == ui.Home || now.Sub(a.lastTouch) < a.cfg.ReturnHome {
		return false
	}
	a.view, a.page = ui.Home, 0
	return true
}

// nextMode cycles through the active modes; "off" has its own button.
func nextMode(modes []string, current string) string {
	active := slices.DeleteFunc(slices.Clone(modes), func(m string) bool { return m == "off" })
	if len(active) == 0 {
		return ""
	}
	return active[(slices.Index(active, current)+1)%len(active)]
}

func (a *App) changeTemp(act ui.Action, st ha.State) {
	value, ok := st.Float("temperature")
	if p := a.temps[act.Entity]; p != nil {
		value, ok = p.value, true
	}
	if !ok {
		return
	}
	step, ok := st.Float("target_temp_step")
	if !ok || step <= 0 {
		step = 1
	}
	if act.Kind == ui.TempDown {
		step = -step
	}
	value += step
	if lo, ok := st.Float("min_temp"); ok {
		value = math.Max(value, lo)
	}
	if hi, ok := st.Float("max_temp"); ok {
		value = math.Min(value, hi)
	}
	now := time.Now()
	a.temps[act.Entity] = &pending{value: value, sendAt: now.Add(tempDelay), until: now.Add(tempExpiry)}
}

// flushTemps sends settled temperatures and drops unconfirmed ones. It
// reports whether the screen needs a redraw.
func (a *App) flushTemps(now time.Time) (dirty bool) {
	for id, p := range a.temps {
		switch {
		case !p.sent && now.After(p.sendAt):
			p.sent = true
			a.call("climate", "set_temperature", id, map[string]any{"temperature": p.value})
		case now.After(p.until):
			delete(a.temps, id)
			dirty = true
		}
	}
	return dirty
}
