package ui

import (
	"fmt"
	"image"
	"strconv"
	"strings"
	"time"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/config"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/ha"
)

// CountdownStep is how often a running countdown changes: often enough to
// follow, rarely enough for an e-ink panel.
const CountdownStep = 5 * time.Second

// duration reads the length of a script's wait: a number of minutes, or the
// entity (such as an input_number) holding it.
func duration(m Model, timer string) (time.Duration, bool) {
	value := timer
	if strings.Contains(timer, ".") {
		if _, err := strconv.ParseFloat(timer, 64); err != nil {
			value = m.States[timer].State
		}
	}
	minutes, err := strconv.ParseFloat(value, 64)
	if err != nil || minutes <= 0 {
		return 0, false
	}
	return time.Duration(minutes * float64(time.Minute)), true
}

// remaining is the time left of a running script, counted from when Home
// Assistant says it was started.
func remaining(m Model, it config.Item, st ha.State) (left, total time.Duration, ok bool) {
	total, ok = duration(m, it.Timer)
	started, err := time.Parse(time.RFC3339Nano, st.String("last_triggered"))
	if !ok || err != nil || st.State != "on" {
		return 0, total, false
	}
	return max(0, started.Add(total).Sub(m.Now)), total, true
}

// clockLeft formats a countdown, rounded up to the step it advances by.
func clockLeft(d time.Duration) string {
	d = (d + CountdownStep - 1) / CountdownStep * CountdownStep
	s := int(d / time.Second)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// drawAction draws a button that runs a script, scene or button entity. While
// a timed script runs it shows the time left.
func drawAction(f *Frame, m Model, b block, name string, st ha.State, available bool) {
	img, r := f.Image, b.rect
	e := b.item.Entity
	script := strings.HasPrefix(e, "script.")
	running := script && available && st.State == "on"
	action := Action{Kind: Run, Entity: e}
	armed := m.Armed == action

	bg, fg, edge := white, black, black
	switch {
	case !available:
		fg, edge, action = midGray, midGray, Action{}
	case running:
		// Tapping again would not restart it, and stopping it could leave
		// whatever it opened as it is.
		bg, fg, action = black, white, Action{}
	case armed:
		bg, fg = black, white
	}
	card(img, r, radius, 3, bg, edge)
	icon(img, iconName(b.item.Icon, st.String("icon"), "play-circle-outline"), 46, r.Min.X+44, (r.Min.Y+r.Max.Y)/2, fg)

	box := image.Rect(r.Min.X+84, r.Min.Y, r.Max.X-18, r.Max.Y)
	mid := (r.Min.Y + r.Max.Y) / 2
	sub := "tocca per avviare"
	if script {
		sub = "premi due volte"
	}
	total, timed := duration(m, b.item.Timer)
	if timed {
		sub += " · " + strconv.FormatFloat(total.Minutes(), 'f', -1, 64) + " min"
	}
	rest, _, counting := remaining(m, b.item, st)
	switch {
	case !available:
		sub = "non disponibile"
	case armed:
		sub = "premi ancora per confermare"
	case counting && rest > 0:
		sub = "in corso, mancano"
	case running:
		sub = "in corso…"
		if counting {
			sub = "sta terminando…"
		}
	}
	text(img, face(true, 26), name, box, mid-15, left, fg)
	text(img, face(false, 20), sub, box, mid+17, left, fg)

	if counting && rest > 0 {
		f.Countdown = true
		clock := clockLeft(rest)
		text(img, face(true, 48), clock, box, mid, right, fg)
		// A bar that empties as the time passes.
		width := int(float64(r.Dx()-36) * min(1, float64(rest)/float64(total)))
		fill(img, image.Rect(r.Min.X+18, r.Max.Y-10, r.Min.X+18+width, r.Max.Y-6), fg)
	}
	f.region(r, radius, action)
}
