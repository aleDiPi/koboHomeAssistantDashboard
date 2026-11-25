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
	OpenCover
	CloseCover
)

// View is the index of one of the pages.
type View int

// The pages of the layout built from the configuration file.
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
	Layout    config.Layout
	States    map[string]ha.State
	History   map[string][]ha.Point
	Connected bool
	Status    string // shown while disconnected
	Now       time.Time
	View      View
	Events    []ha.Event
	Page      int
	// Target temperatures chosen by the user and not yet confirmed.
	Pending map[string]float64
	// Armed is the action waiting for a confirming second touch.
	Armed   Action
	Battery int // percent, negative if unknown
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
	windowLow    = 140
	radius       = 18
)

type blockKind int

const (
	sensorBlock blockKind = iota
	lightBlock
	climateBlock
	windowBlock
	calendarBlock
)

type block struct {
	kind blockKind
	item config.Item
	rect image.Rectangle
	page int
}

// body is the area between the header and the navigation bar.
func body(w, h int) image.Rectangle {
	return image.Rect(margin, headerHeight+gap, w-margin, h-navHeight-2*margin)
}

// flow places the content of a page: each kind of entity in its own grid, one
// under the other, the calendar taking what is left. What does not fit goes to
// further sheets.
func flow(p config.Page, area image.Rectangle, landscape bool) (blocks []block, sheets int) {
	type section struct {
		kind                  blockKind
		items                 []config.Item
		cols, lowest, highest int
	}
	cols := func(wide, tall int) int {
		if landscape {
			return wide
		}
		return tall
	}
	var sections []section
	for _, s := range []section{
		{sensorBlock, p.Sensors, 4, sensorHeight, sensorHeight + 20},
		{lightBlock, p.Lights, cols(4, 3), lightHeight, 190},
		{climateBlock, p.Climate, cols(2, 1), climateH, 200},
		{windowBlock, p.Windows, cols(2, 1), windowLow, 170},
	} {
		if len(s.items) > 0 {
			sections = append(sections, s)
		}
	}

	sheet, y := 0, area.Min.Y
	for n, s := range sections {
		// Only the last grid may stretch over the remaining space.
		if n < len(sections)-1 || len(p.Calendars) > 0 {
			s.highest = min(s.highest, s.lowest+20)
		}
		if area.Max.Y-y < s.lowest {
			sheet, y = sheet+1, area.Min.Y
		}
		rows := (len(s.items) + s.cols - 1) / s.cols
		space := area.Max.Y - y
		height := max(s.lowest, min(s.highest, (space-(rows-1)*gap)/rows))
		width := (area.Dx() - (s.cols-1)*gap) / s.cols
		for i := 0; i < len(s.items); i += s.cols {
			if y+height > area.Max.Y && y > area.Min.Y {
				sheet, y = sheet+1, area.Min.Y
			}
			for j := 0; j < s.cols && i+j < len(s.items); j++ {
				x := area.Min.X + j*(width+gap)
				blocks = append(blocks, block{s.kind, s.items[i+j], image.Rect(x, y, x+width, y+height), sheet})
			}
			y += height + gap
		}
	}
	if len(p.Calendars) > 0 {
		if area.Max.Y-y < 300 && y > area.Min.Y {
			sheet, y = sheet+1, area.Min.Y
		}
		blocks = append(blocks, block{kind: calendarBlock, rect: image.Rect(area.Min.X, y, area.Max.X, area.Max.Y), page: sheet})
	}
	return blocks, sheet + 1
}

// Render draws one sheet of the current page on a w×h canvas.
func Render(m Model, w, h int) Frame {
	img := image.NewGray(image.Rect(0, 0, w, h))
	fill(img, img.Rect, white)

	var page config.Page
	view := View(0)
	if n := len(m.Layout.Pages); n > 0 {
		view = View(min(max(int(m.View), 0), n-1))
		page = m.Layout.Pages[view]
	}
	blocks, pages := flow(page, body(w, h), w > h)
	f := Frame{Image: img, View: view, Pages: pages, Page: m.Page % pages}

	drawHeader(&f, m, w)
	for _, b := range blocks {
		if b.page != f.Page {
			continue
		}
		if b.kind == calendarBlock {
			drawCalendar(&f, m, page.Calendars, b.rect)
			continue
		}
		st, known := m.States[b.item.Entity]
		available := m.Connected && known && st.State != "unavailable" && st.State != "unknown"
		name := b.item.Name
		if name == "" {
			name = st.String("friendly_name")
		}
		if name == "" {
			name = b.item.Entity
		}
		switch b.kind {
		case sensorBlock:
			drawSensor(&f, m, b, name, st, available)
		case lightBlock:
			// Scenes and buttons have no state of their own until first used.
			available = available || (m.Connected && known && momentary(b.item.Entity) && st.State == "unknown")
			drawLight(&f, b, name, st, available)
		case climateBlock:
			drawClimate(&f, m, b, name, st, available)
		case windowBlock:
			drawWindow(&f, m, b, name, st, available)
		}
	}
	drawNav(&f, m, w, h)
	return f
}

// momentary entities are run rather than switched on and off.
func momentary(entity string) bool {
	domain, _, _ := strings.Cut(entity, ".")
	return domain == "scene" || domain == "script" || domain == "button" || domain == "input_button"
}

// drawNav draws the buttons that switch page, with what is on in each.
func drawNav(f *Frame, m Model, w, h int) {
	n := len(m.Layout.Pages)
	if n < 2 {
		return
	}
	count := func(items []config.Item, active func(string) bool) (n int) {
		for _, it := range items {
			if st, ok := m.States[it.Entity]; ok && active(st.State) {
				n++
			}
		}
		return n
	}
	width := (w - 2*margin - (n-1)*gap) / n
	for i, p := range m.Layout.Pages {
		label, on, one, many := p.Title, 0, "", ""
		switch {
		case len(p.Lights) > 0:
			on, one, many = count(p.Lights, func(s string) bool { return s == "on" }), "accesa", "accese"
		case len(p.Climate) > 0:
			off := func(s string) bool { return s == "off" || s == "unavailable" || s == "unknown" }
			on, one, many = count(p.Climate, func(s string) bool { return !off(s) }), "acceso", "accesi"
		}
		if m.Connected && on > 0 {
			word := many
			if on == 1 {
				word = one
			}
			label = fmt.Sprintf("%s · %d %s", p.Title, on, word)
		}
		x := margin + i*(width+gap)
		r := image.Rect(x, h-margin-navHeight, x+width, h-margin)
		pill(f, r, label, View(i) == f.View, true, Action{Kind: Show, View: View(i)})
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
	armed := m.Armed.Kind == Exit
	if armed {
		label = "Sicuro?"
	}
	pill(f, exit, label, armed, true, Action{Kind: Exit})
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
	for _, it := range m.Layout.Notifications {
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
	fallback := "lightbulb"
	if len(st.Strings("entity_id")) > 0 {
		fallback = "lightbulb-group"
	}
	symbol := stateIcon(iconName(b.item.Icon, st.String("icon"), fallback), on)
	bg, fg, edge := white, black, darkGray
	action := Action{Kind: Toggle, Entity: b.item.Entity}
	offWord, onWord := "spento", "acceso"
	if strings.HasPrefix(b.item.Entity, "light.") {
		offWord, onWord = "spenta", "accesa"
	}
	if momentary(b.item.Entity) {
		offWord = "tocca per avviare"
	}
	state := offWord
	switch {
	case !available:
		state, fg, edge, action = "non disp.", midGray, midGray, Action{}
	case on:
		state, bg, fg, edge = onWord, black, white, black
	}
	card(img, r, radius, 3, bg, edge)
	if r.Dy() >= 130 {
		// A tall tile stacks symbol, name and state.
		icon(img, symbol, 72, (r.Min.X+r.Max.X)/2, r.Min.Y+r.Dy()*30/100, fg)
		box := r.Inset(10)
		text(img, face(true, 26), name, box, r.Min.Y+r.Dy()*64/100, center, fg)
		text(img, face(false, 20), state, box, r.Min.Y+r.Dy()*84/100, center, fg)
		f.region(r, radius, action)
		return
	}
	icon(img, symbol, 46, r.Min.X+38, r.Min.Y+r.Dy()/2, fg)

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
