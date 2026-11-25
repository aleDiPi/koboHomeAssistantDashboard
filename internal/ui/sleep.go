package ui

import (
	"fmt"
	"image"
	"strconv"
	"strings"
)

// special is the view of the screens that are not pages.
const special View = -1

func blank(w, h int) Frame {
	img := image.NewGray(image.Rect(0, 0, w, h))
	fill(img, img.Rect, white)
	return Frame{Image: img, View: special, Pages: 1}
}

// RenderSleep draws what stays on screen while the device is suspended: that it
// is asleep and how to wake it, with the readings of the last wake.
func RenderSleep(m Model, w, h int) Frame {
	f := blank(w, h)
	img := f.Image
	all := image.Rect(24, 0, w-24, h)

	icon(img, "sleep", 120, w/2, h*16/100, black)
	text(img, face(true, 54), "In standby", all, h*31/100, center, black)
	text(img, face(false, 28), "Premi il tasto di accensione per usare la dashboard", all, h*39/100, center, black)

	// The first page's sensors, as large figures.
	if len(m.Layout.Pages) > 0 {
		sensors := m.Layout.Pages[0].Sensors
		if n := min(len(sensors), 4); n > 0 {
			width := (w - 48) / n
			for i, it := range sensors[:n] {
				st := m.States[it.Entity]
				value := "--"
				if v, err := strconv.ParseFloat(st.State, 64); err == nil {
					value = number(v)
					if strings.HasPrefix(st.String("unit_of_measurement"), "°") {
						value += "°"
					}
				}
				name := it.Name
				if name == "" {
					name = st.String("friendly_name")
				}
				cell := image.Rect(24+i*width, 0, 24+(i+1)*width, h)
				text(img, face(true, 52), value, cell, h*55/100, center, black)
				text(img, face(false, 22), name, cell.Inset(6), h*62/100, center, darkGray)
			}
		}
	}

	if labels := notices(m); len(labels) > 0 {
		text(img, face(true, 26), strings.Join(labels, " · "), all, h*76/100, center, black)
	}
	foot := "Ultimo aggiornamento " + m.Now.Format("15:04")
	if m.Battery >= 0 {
		foot += fmt.Sprintf(" · batteria %d%%", m.Battery)
	}
	text(img, face(false, 22), foot, all, h*90/100, center, darkGray)
	return f
}

// RenderWaking answers the power button at once, while Wi-Fi comes back.
func RenderWaking(w, h int) Frame {
	f := blank(w, h)
	all := image.Rect(24, 0, w-24, h)
	icon(f.Image, "power", 120, w/2, h*30/100, black)
	text(f.Image, face(true, 54), "Mi sto riattivando…", all, h*52/100, center, black)
	text(f.Image, face(false, 28), "Attendi la connessione Wi-Fi", all, h*62/100, center, darkGray)
	return f
}
