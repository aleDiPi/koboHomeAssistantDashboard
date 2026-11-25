// Package ui lays out and draws the dashboard.
package ui

import (
	"fmt"
	"image"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/config"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/ha"
)

// Kind says what a tap does.
type Kind int

const (
	None Kind = iota
	Toggle
	Power
	TempDown
	TempUp
	Mode
	NextPage
	Exit
	Show // switch to another view
)

// View is one of the screens.
type View int

const (
	Home View = iota
	Lights
	Climate
)

type Action struct {
	Kind   Kind
	Entity string
	View   View // for Show
}

// Region is one independently refreshed area of the screen.
type Region struct {
	Rect   image.Rectangle
	Radius int    // corner radius of the shape drawn there
	Action Action // Kind None for areas that cannot be tapped
}

// Model is everything a frame depends on.
type Model struct {
	Config    config.Config
	States    map[string]ha.State
	History   map[string][]ha.Point
	Connected bool
	Status    string // shown while disconnected
	Now       time.Time
	View      View
	Events    []ha.Event
	Page      int
	// Target temperatures chosen by the user and not yet confirmed.
	Pending   map[string]float64
	ExitArmed bool
	Battery   int // percent, negative if unknown
}

type Frame struct {
	Image   *image.Gray
	Regions []Region
	View    View
	Page    int
	Pages   int
}

const (
	margin       = 8
	gap          = 8
	headerHeight = 70
	sensorHeight = 168
	lightHeight  = 96
	climateH     = 164
	navHeight    = 68
	radius       = 18
)

type block struct {
	item config.Item
	rect image.Rectangle
	page int
}

// body is the area between the header and the navigation bar.
func body(w, h int) image.Rectangle {
	return image.Rect(margin, headerHeight+gap, w-margin, h-navHeight-2*margin)
}

// grid flows items into rows of cols and the rows into pages. Rows grow to
// share the height of area, between lowest and highest.
func grid(items []config.Item, area image.Rectangle, cols, lowest, highest int) (blocks []block, pages int) {
	if len(items) == 0 {
		return nil, 1
	}
	rows := (len(items) + cols - 1) / cols
	height := max(lowest, min(highest, (area.Dy()-(rows-1)*gap)/rows))
	perPage := max(1, (area.Dy()+gap)/(height+gap))
	width := (area.Dx() - (cols-1)*gap) / cols
	for i, it := range items {
		row := i / cols
		x := area.Min.X + (i%cols)*(width+gap)
		y := area.Min.Y + (row%perPage)*(height+gap)
		blocks = append(blocks, block{it, image.Rect(x, y, x+width, y+height), row / perPage})
	}
	return blocks, (rows-1)/perPage + 1
}

// Render draws one page of the current view on a w×h canvas.
func Render(m Model, w, h int) Frame {
	img := image.NewGray(image.Rect(0, 0, w, h))
	fill(img, img.Rect, white)

	area := body(w, h)
	landscape := w > h
	var blocks []block
	pages := 1
	switch m.View {
	case Lights:
		cols := 3
		if landscape {
			cols = 4
		}
		blocks, pages = grid(m.Config.Lights, area, cols, lightHeight, 190)
	case Climate:
		cols := 1
		if landscape {
			cols = 2
		}
		blocks, pages = grid(m.Config.Climate, area, cols, climateH, 220)
	default:
		blocks, _ = grid(m.Config.Sensors, area, 4, sensorHeight, sensorHeight+20)
	}
	f := Frame{Image: img, View: m.View, Pages: pages, Page: m.Page % pages}

	drawHeader(&f, m, w)
	below := area.Min.Y
	for _, b := range blocks {
		if b.page != f.Page {
			continue
		}
		below = max(below, b.rect.Max.Y+gap)
		st, known := m.States[b.item.Entity]
		available := m.Connected && known && st.State != "unavailable" && st.State != "unknown"
		name := b.item.Name
		if name == "" {
			name = st.String("friendly_name")
		}
		if name == "" {
			name = b.item.Entity
		}
		switch m.View {
		case Lights:
			drawLight(&f, b, name, st, available)
		case Climate:
			drawClimate(&f, m, b, name, st, available)
		default:
			drawSensor(&f, m, b, name, st, available)
		}
	}
	if m.View == Home && area.Max.Y-below > 200 {
		drawCalendar(&f, m, image.Rect(area.Min.X, below, area.Max.X, area.Max.Y))
	}
	drawNav(&f, m, w, h)
	return f
}

// drawNav draws the buttons that switch view, with what is on in each.
func drawNav(f *Frame, m Model, w, h int) {
	count := func(items []config.Item, active func(string) bool) (n int) {
		for _, it := range items {
			if st, ok := m.States[it.Entity]; ok && active(st.State) {
				n++
			}
		}
		return n
	}
	label := func(name string, n int, one, many string) string {
		switch {
		case !m.Connected || n == 0:
			return name
		case n == 1:
			return name + " · 1 " + one
		}
		return fmt.Sprintf("%s · %d %s", name, n, many)
	}
	lights := count(m.Config.Lights, func(s string) bool { return s == "on" })
	climate := count(m.Config.Climate, func(s string) bool { return s != "off" && s != "unavailable" && s != "unknown" })
	labels := []string{
		"Casa",
		label("Luci", lights, "accesa", "accese"),
		label("Clima", climate, "acceso", "accesi"),
	}
	width := (w - 2*margin - 2*gap) / len(labels)
	for i, l := range labels {
		x := margin + i*(width+gap)
		r := image.Rect(x, h-margin-navHeight, x+width, h-margin)
		pill(f, r, l, View(i) == m.View, true, Action{Kind: Show, View: View(i)})
	}
}

func (f *Frame) region(r image.Rectangle, rad int, a Action) {
	f.Regions = append(f.Regions, Region{Rect: r, Radius: rad, Action: a})
}

var (
	weekdays = [...]string{"dom", "lun", "mar", "mer", "gio", "ven", "sab"}
	months   = [...]string{"gen", "feb", "mar", "apr", "mag", "giu", "lug", "ago", "set", "ott", "nov", "dic"}
)

func drawHeader(f *Frame, m Model, w int) {
	img := f.Image
	mid := headerHeight / 2

	clock := image.Rect(margin+4, 0, 150, headerHeight)
	text(img, face(true, 44), m.Now.Format("15:04"), clock, mid, left, black)
	f.region(clock, 0, Action{})

	day := fmt.Sprintf("%s %d %s", weekdays[m.Now.Weekday()], m.Now.Day(), months[m.Now.Month()-1])
	date := image.Rect(clock.Max.X+4, 0, clock.Max.X+4+measure(face(false, 22), day), headerHeight)
	text(img, face(false, 22), day, date, mid, left, darkGray)
	f.region(date, 0, Action{})

	exit := image.Rect(w-margin-112, 8, w-margin, headerHeight-6)
	label := "Esci"
	if m.ExitArmed {
		label = "Sicuro?"
	}
	pill(f, exit, label, m.ExitArmed, true, Action{Kind: Exit})
	edge := exit.Min.X - gap
	if f.Pages > 1 {
		next := exit.Sub(image.Pt(exit.Dx()+gap, 0))
		pill(f, next, fmt.Sprintf("Pag %d/%d", f.Page+1, f.Pages), false, true, Action{Kind: NextPage})
		edge = next.Min.X - gap
	}

	if m.Battery >= 0 {
		battery := image.Rect(edge-110, 0, edge, headerHeight)
		drawBattery(img, image.Rect(battery.Min.X, mid-9, battery.Min.X+34, mid+9), m.Battery)
		text(img, face(false, 22), fmt.Sprintf("%d%%", m.Battery),
			image.Rect(battery.Min.X+44, 0, battery.Max.X, headerHeight), mid, left, darkGray)
		f.region(battery, 0, Action{})
		edge = battery.Min.X - gap
	}

	status := image.Rect(date.Max.X+14, 0, edge, headerHeight)
	if status.Empty() {
		return
	}
	if !m.Connected {
		msg := m.Status
		if msg == "" {
			msg = "Non connesso"
		}
		card(img, image.Rect(status.Min.X, 14, status.Max.X, headerHeight-12), 22, 0, black, black)
		text(img, face(true, 22), msg, status.Inset(10), mid, center, white)
	} else {
		drawChips(img, status, notices(m))
	}
	f.region(status, 0, Action{})
}

// notices lists the labels of the notifications that are active now.
func notices(m Model) []string {
	var out []string
	for _, it := range m.Config.Notifications {
		st := m.States[it.Entity]
		if st.State != "on" && st.State != "open" {
			continue
		}
		name := it.Name
		if name == "" {
			name = st.String("friendly_name")
		}
		if name == "" {
			name = it.Entity
		}
		out = append(out, name)
	}
	return out
}

// drawChips puts one pill per notice in r; those that do not fit are counted
// in a last "+N" pill.
func drawChips(img *image.Gray, r image.Rectangle, labels []string) {
	const pad, space, countWidth = 14, 8, 56
	f := face(true, 19)
	x := r.Min.X
	chip := func(label string, width int) {
		box := image.Rect(x, 16, x+width, headerHeight-14)
		card(img, box, float64(box.Dy())/2, 0, black, black)
		text(img, f, label, box.Inset(6), headerHeight/2-1, center, white)
		x = box.Max.X + space
	}
	for i, label := range labels {
		width := measure(f, label) + 2*pad
		room := r.Max.X - x
		if i < len(labels)-1 {
			room -= countWidth + space
		}
		if width > room {
			// A single long notice is shortened rather than hidden.
			if i == 0 && room > 100 {
				chip(label, room)
				i++
			}
			if rest := len(labels) - i; rest > 0 {
				chip(fmt.Sprintf("+%d", rest), countWidth)
			}
			return
		}
		chip(label, width)
	}
}

func drawBattery(img *image.Gray, r image.Rectangle, percent int) {
	card(img, r, 4, 2, white, black)
	fill(img, image.Rect(r.Max.X, r.Min.Y+5, r.Max.X+3, r.Max.Y-5), black)
	inner := r.Inset(4)
	inner.Max.X = inner.Min.X + inner.Dx()*min(percent, 100)/100
	fill(img, inner, black)
}

// pill draws a rounded button. A disabled button is greyed out and inert.
func pill(f *Frame, r image.Rectangle, label string, inverted, enabled bool, a Action) {
	bg, fg, edge := white, black, black
	if inverted {
		bg, fg = black, white
	}
	if !enabled {
		fg, edge, a = midGray, midGray, Action{}
	}
	rad := r.Dy() / 2
	card(f.Image, r, float64(rad), 3, bg, edge)
	size := 24
	inner := r.Inset(8)
	for size > 16 && measure(face(true, size), label) > inner.Dx() {
		size -= 2
	}
	text(f.Image, face(true, size), label, inner, (r.Min.Y+r.Max.Y)/2, center, fg)
	f.region(r, rad, a)
}

// round draws a circular button of diameter d with a symbol inside.
func round(f *Frame, x, y, d int, inverted, enabled bool, a Action, symbol func(cx, cy float64, g uint8)) {
	r := image.Rect(x, y, x+d, y+d)
	bg, fg, edge := white, black, black
	if inverted {
		bg, fg = black, white
	}
	if !enabled {
		fg, edge, a = midGray, midGray, Action{}
	}
	card(f.Image, r, float64(d)/2, 3, bg, edge)
	symbol(float64(x)+float64(d)/2, float64(y)+float64(d)/2, fg)
	f.region(r, d/2, a)
}

func drawSensor(f *Frame, m Model, b block, name string, st ha.State, available bool) {
	img, r := f.Image, b.rect
	card(img, r, radius, 2, white, midGray)
	value := "--"
	current, numeric := 0.0, false
	if available {
		value = st.State
		if v, err := strconv.ParseFloat(st.State, 64); err == nil {
			current, numeric, value = v, true, number(v)
		}
		unit := st.String("unit_of_measurement")
		if strings.HasPrefix(unit, "°") {
			unit = "°"
		}
		value += unit
	}
	inner := r.Inset(14)
	text(img, face(false, 20), name, inner, r.Min.Y+22, left, darkGray)
	text(img, face(true, 46), value, inner, r.Min.Y+60, left, black)

	graph := image.Rect(inner.Min.X, r.Min.Y+90, inner.Max.X, r.Max.Y-30)
	points := m.History[b.item.Entity]
	if numeric {
		points = append(points[:len(points):len(points)], ha.Point{Time: m.Now, Value: current})
	}
	if low, high, ok := sparkline(img, graph, points, m.Now); ok {
		small := face(false, 17)
		foot := r.Max.Y - 16
		text(img, small, "min "+number(low), inner, foot, left, darkGray)
		text(img, small, "max "+number(high), inner, foot, right, darkGray)
	}
	f.region(r, radius, Action{})
}

// sparkline plots the last 24 hours as a filled curve. The scale spans at
// least one degree, so that sensor noise does not look like a swing.
func sparkline(img *image.Gray, r image.Rectangle, points []ha.Point, now time.Time) (low, high float64, ok bool) {
	if len(points) < 2 {
		return 0, 0, false
	}
	start := now.Add(-ha.HistorySpan)
	values := make([]float64, r.Dx()/3+1)
	i, last := 0, points[0].Value
	for col := range values {
		until := start.Add(time.Duration(float64(ha.HistorySpan) * float64(col) / float64(len(values)-1)))
		for i < len(points) && !points[i].Time.After(until) {
			last = points[i].Value
			i++
		}
		values[col] = last
	}
	// The newest reading closes the curve even if its clock is slightly ahead.
	values[len(values)-1] = points[len(points)-1].Value
	low, high = slices.Min(values), slices.Max(values)
	bottom, top := low, high
	if top-bottom < 1 {
		mean := (top + bottom) / 2
		bottom, top = mean-0.5, mean+0.5
	}
	at := func(col int) (float64, float64) {
		x := float64(r.Min.X) + float64(col)*float64(r.Dx()-1)/float64(len(values)-1)
		y := float64(r.Max.Y-2) - (values[col]-bottom)/(top-bottom)*float64(r.Dy()-4)
		return x, y
	}
	for col := 1; col < len(values); col++ {
		x0, y0 := at(col - 1)
		x1, y1 := at(col)
		for x := int(math.Ceil(x0)); x < int(math.Ceil(x1)); x++ {
			y := y0 + (y1-y0)*(float64(x)-x0)/(x1-x0)
			fill(img, image.Rect(x, int(y), x+1, r.Max.Y), lightGray)
		}
	}
	for col := 1; col < len(values); col++ {
		x0, y0 := at(col - 1)
		x1, y1 := at(col)
		line(img, x0, y0, x1, y1, 3, black)
	}
	return low, high, true
}

func drawLight(f *Frame, b block, name string, st ha.State, available bool) {
	img, r := f.Image, b.rect
	on := available && st.State == "on"
	bg, fg, edge := white, black, darkGray
	action := Action{Kind: Toggle, Entity: b.item.Entity}
	state := "spenta"
	switch {
	case !available:
		state, fg, edge, action = "non disp.", midGray, midGray, Action{}
	case on:
		state, bg, fg, edge = "accesa", black, white, black
	}
	card(img, r, radius, 3, bg, edge)
	if r.Dy() >= 130 {
		// A tall tile stacks symbol, name and state.
		bulb(img, float64(r.Min.X+r.Max.X)/2, float64(r.Min.Y)+float64(r.Dy())*0.28, on, fg)
		box := r.Inset(10)
		text(img, face(true, 26), name, box, r.Min.Y+r.Dy()*64/100, center, fg)
		text(img, face(false, 20), state, box, r.Min.Y+r.Dy()*84/100, center, fg)
		f.region(r, radius, action)
		return
	}
	bulb(img, float64(r.Min.X+38), float64(r.Min.Y+r.Dy()/2), on, fg)

	box := image.Rect(r.Min.X+72, r.Min.Y, r.Max.X-10, r.Max.Y)
	big := face(true, 24)
	lines := wrap(big, name, box.Dx())
	y := r.Min.Y + 36 - (len(lines)-1)*13
	for i, l := range lines {
		text(img, big, l, box, y+i*27, left, fg)
	}
	text(img, face(false, 19), state, box, r.Max.Y-22, left, fg)
	f.region(r, radius, action)
}

var modeNames = map[string]string{
	"off": "Spento", "cool": "Freddo", "heat": "Caldo", "dry": "Deumid.",
	"fan_only": "Ventola", "auto": "Auto", "heat_cool": "Auto",
}

func modeName(m string) string {
	if n, ok := modeNames[m]; ok {
		return n
	}
	return m
}

func number(v float64) string {
	return strings.Replace(strconv.FormatFloat(v, 'f', 1, 64), ".", ",", 1)
}

func drawClimate(f *Frame, m Model, b block, name string, st ha.State, available bool) {
	img, r := f.Image, b.rect
	card(img, r, radius, 2, white, midGray)

	on := available && st.State != "off"
	title := image.Rect(r.Min.X+14, r.Min.Y+8, r.Max.X-14, r.Min.Y+66)
	target, hasTarget := st.Float("temperature")
	if v, ok := m.Pending[b.item.Entity]; ok {
		target, hasTarget = v, true
	}
	sub := "non disponibile"
	if available {
		sub = modeName(st.State)
		if cur, ok := st.Float("current_temperature"); ok {
			sub += " · ora " + number(cur) + "°"
		}
	}
	text(img, face(true, 27), name, title, r.Min.Y+27, left, black)
	text(img, face(false, 20), sub, title, r.Min.Y+53, left, darkGray)
	f.region(title, 0, Action{})

	const d = 76
	e := b.item.Entity
	y := r.Max.Y - d - 12
	x := r.Min.X + 12
	adjust := available && hasTarget
	sign := func(plus bool) func(cx, cy float64, g uint8) {
		return func(cx, cy float64, g uint8) {
			line(img, cx-14, cy, cx+14, cy, 6, g)
			if plus {
				line(img, cx, cy-14, cx, cy+14, 6, g)
			}
		}
	}
	round(f, x, y, d, false, adjust, Action{Kind: TempDown, Entity: e}, sign(false))

	value := image.Rect(x+d+4, y, x+d+4+124, y+d)
	targetText := "--"
	if adjust {
		targetText = number(target) + "°"
	}
	text(img, face(true, 42), targetText, value, y+d/2, center, black)
	f.region(value, 0, Action{})
	round(f, value.Max.X+4, y, d, false, adjust, Action{Kind: TempUp, Entity: e}, sign(true))

	power := r.Max.X - 12 - d
	mode := image.Rect(value.Max.X+4+d+10, y+10, power-8, y+d-10)
	label := "Modo"
	if on {
		label = modeName(st.State)
	}
	pill(f, mode, label, false, on, Action{Kind: Mode, Entity: e})
	round(f, power, y, d, on, available, Action{Kind: Power, Entity: e}, func(cx, cy float64, g uint8) {
		powerSymbol(img, cx, cy, g)
	})
}

// Hit returns the action under p.
func (f Frame) Hit(p image.Point) (Region, bool) {
	for _, r := range f.Regions {
		if r.Action.Kind != None && p.In(r.Rect) {
			return r, true
		}
	}
	return Region{}, false
}

// Invert flips the pixels of a region's shape, as feedback for a touch.
func Invert(img *image.Gray, r Region) {
	shape := roundRect(r.Rect, float64(r.Radius))
	area := r.Rect.Intersect(img.Rect)
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			if shape(float64(x)+0.5, float64(y)+0.5) <= 0 {
				i := img.PixOffset(x, y)
				img.Pix[i] = 255 - img.Pix[i]
			}
		}
	}
}
