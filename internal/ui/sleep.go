package ui

import (
	"fmt"
	"image"
	"strconv"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// special is the view of the screens that are not pages.
const special View = -1

func blank(w, h int) Frame {
	img := image.NewGray(image.Rect(0, 0, w, h))
	fill(img, img.Rect, white)
	return Frame{Image: img, View: special, Pages: 1}
}

// reading is a temperature for the standby screen, or "" if not configured.
func reading(m Model, entity, label string) string {
	if entity == "" {
		return ""
	}
	value := "--"
	if v, err := strconv.ParseFloat(m.States[entity].State, 64); err == nil {
		value = number(v) + "°"
	}
	return label + " " + value
}

// tag writes s on a white pill, so that it reads over any picture.
func tag(img *image.Gray, s string, size, x, y int, a align) {
	f := face(true, size)
	width, height := measure(f, s)+32, size+22
	switch a {
	case right:
		x -= width
	case center:
		x -= width / 2
	}
	r := image.Rect(x, y, x+width, y+height)
	card(img, r, float64(height)/2, 2, white, black)
	text(img, f, s, r.Inset(6), y+height/2, center, black)
}

// RenderSleep draws what stays on screen while the device is suspended: a
// picture, the temperatures of the last wake in small, and how to wake it.
func RenderSleep(m Model, w, h int) Frame {
	f := blank(w, h)
	img := f.Image
	indoor := reading(m, m.Config.Indoor, "Sala")
	outdoor := reading(m, m.Config.Outdoor, "Fuori")

	if p := m.Picture; p != nil {
		at := image.Pt((w-p.Rect.Dx())/2, (h-p.Rect.Dy())/2)
		xdraw.Draw(img, p.Rect.Add(at), p, image.Point{}, xdraw.Src)
	} else {
		icon(img, "sleep", 140, w/2, h*35/100, black)
		text(img, face(true, 54), "In standby", img.Rect, h*55/100, center, black)
	}

	if indoor != "" {
		tag(img, indoor, 34, 16, 16, left)
	}
	if outdoor != "" {
		tag(img, outdoor, 26, w-16, 16, right)
	}
	// Alerts, such as an open window, only while there is one.
	if labels := notices(m); len(labels) > 0 {
		tag(img, strings.Join(labels, " · "), 22, w/2, h-96, center)
	}
	foot := "Premi il tasto di accensione · " + m.Now.Format("15:04")
	if m.Battery >= 0 {
		foot += fmt.Sprintf(" · %d%%", m.Battery)
	}
	tag(img, foot, 19, w/2, h-48, center)
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
