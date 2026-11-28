package ui

import (
	"fmt"
	"image"
	"strings"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/ha"
)

// windowState describes a contact sensor or a motorised window.
func windowState(st ha.State) (label string, open bool) {
	switch st.State {
	case "off", "closed":
		return "Chiusa", false
	case "on":
		return "Aperta", true
	case "opening":
		return "Si sta aprendo", true
	case "closing":
		return "Si sta chiudendo", true
	case "open":
		if p, ok := st.Float("current_position"); ok && p < 100 {
			return fmt.Sprintf("Aperta al %.0f%%", p), true
		}
		return "Aperta", true
	}
	return "non disponibile", false
}

// drawWindow shows whether a window is open; a motorised one (cover) also gets
// buttons to move it.
func drawWindow(f *Frame, m Model, b block, name string, st ha.State, available bool) {
	img, r := f.Image, b.rect
	e := b.item.Entity
	cover := strings.HasPrefix(e, "cover.")
	state, open := windowState(st)
	class := st.String("device_class")
	opening := cover || class == "window" || class == "door" || class == "opening" || class == "garage_door"
	if !opening {
		// Any other binary sensor.
		state = "Non attivo"
		if open {
			state = "Attivo"
		}
	}
	if !available {
		state, open = "non disponibile", false
	}
	card(img, r, radius, 2, white, midGray)

	// The upper part, or the whole card for a sensor: symbol, name, state.
	info := r
	if cover {
		info.Max.Y = r.Max.Y - 70
	}
	cy := (info.Min.Y + info.Max.Y) / 2
	badge := image.Rect(r.Min.X+14, cy-32, r.Min.X+78, cy+32)
	bg, fg := white, black
	symbol := "window-closed-variant"
	if open {
		bg, fg, symbol = black, white, "window-open-variant"
	}
	if !available {
		fg = midGray
	}
	card(img, badge, 32, 0, bg, bg)
	if !opening {
		symbol = iconName(b.item.Icon, st.String("icon"), "radiobox-blank")
	}
	icon(img, symbol, 44, badge.Min.X+32, cy, fg)
	box := image.Rect(badge.Max.X+14, info.Min.Y, r.Max.X-12, info.Max.Y)
	text(img, face(true, 26), name, box, cy-14, left, black)
	text(img, face(open, 21), state, box, cy+17, left, darkGray)
	f.region(info.Inset(3), 0, Action{})

	if !cover {
		return
	}
	width := (r.Dx() - 3*12) / 2
	button := func(i int) image.Rectangle {
		x := r.Min.X + 12 + i*(width+12)
		return image.Rect(x, r.Max.Y-66, x+width, r.Max.Y-12)
	}
	// Opening a window asks for a second touch.
	openAction := Action{Kind: OpenCover, Entity: e}
	label, armed := "Apri", m.Armed == openAction
	if armed {
		label = "Sicuro?"
	}
	pill(f, button(0), label, armed, available, openAction)
	pill(f, button(1), "Chiudi", false, available, Action{Kind: CloseCover, Entity: e})
}
