// Package ui lays out and draws the dashboard.
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
)

type Action struct {
	Kind   Kind
	Entity string
}

// Region is one independently refreshed area of the screen.
type Region struct {
	Rect   image.Rectangle
	Action Action // Kind None for areas that cannot be tapped
}

// Model is everything a frame depends on.
type Model struct {
	Config    config.Config
	States    map[string]ha.State
	Connected bool
	Status    string // shown while disconnected
	Now       time.Time
	Page      int
	// Target temperatures chosen by the user and not yet confirmed.
	Pending   map[string]float64
	ExitArmed bool
}

type Frame struct {
	Image   *image.Gray
	Regions []Region
	Page    int
	Pages   int
}

const (
	margin       = 8
	gap          = 8
	headerHeight = 72
	sensorHeight = 112
	lightHeight  = 104
	infoHeight   = 76
	buttonHeight = 88
)

type blockKind int

const (
	sensorBlock blockKind = iota
	lightBlock
	climateBlock
)

type block struct {
	kind blockKind
	item config.Item
	rect image.Rectangle
	page int
}

// layout flows the entities into rows and the rows into pages.
func layout(c config.Config, w, h int) (blocks []block, pages int) {
	sensorCols, lightCols, climateCols := 4, 3, 1
	if w > h {
		sensorCols, lightCols, climateCols = 5, 4, 2
	}
	top := headerHeight + gap
	y, page := top, 0
	section := func(kind blockKind, items []config.Item, cols, height int) {
		width := (w - 2*margin - (cols-1)*gap) / cols
		for i := 0; i < len(items); i += cols {
			if y+height > h-margin && y > top {
				y, page = top, page+1
			}
			for j := 0; j < cols && i+j < len(items); j++ {
				x := margin + j*(width+gap)
				blocks = append(blocks, block{kind, items[i+j], image.Rect(x, y, x+width, y+height), page})
			}
			y += height + gap
		}
	}
	section(sensorBlock, c.Sensors, sensorCols, sensorHeight)
	section(lightBlock, c.Lights, lightCols, lightHeight)
	section(climateBlock, c.Climate, climateCols, infoHeight+buttonHeight)
	return blocks, page + 1
}

// Render draws one page on a w×h canvas.
func Render(m Model, w, h int) Frame {
	img := image.NewGray(image.Rect(0, 0, w, h))
	fill(img, img.Rect, white)

	blocks, pages := layout(m.Config, w, h)
	f := Frame{Image: img, Pages: pages, Page: m.Page % pages}

	drawHeader(&f, m, w)
	for _, b := range blocks {
		if b.page != f.Page {
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
			drawSensor(&f, b, name, st, available)
		case lightBlock:
			drawLight(&f, b, name, st, available)
		case climateBlock:
			drawClimate(&f, m, b, name, st, available)
		}
	}
	return f
}

func drawHeader(f *Frame, m Model, w int) {
	img := f.Image
	const buttonW = 120
	clock := image.Rect(margin, 0, 200, headerHeight-2)
	text(img, face(true, 46), m.Now.Format("15:04"), clock, headerHeight/2, left, black)
	f.Regions = append(f.Regions, Region{Rect: clock})

	exit := image.Rect(w-margin-buttonW, 6, w-margin, headerHeight-8)
	statusRight := exit.Min.X - gap
	label := "Esci"
	if m.ExitArmed {
		label = "Sicuro?"
	}
	button(f, exit, label, m.ExitArmed, true, Action{Kind: Exit})

	if f.Pages > 1 {
		next := exit.Sub(image.Pt(buttonW+gap, 0))
		button(f, next, fmt.Sprintf("Pag %d/%d", f.Page+1, f.Pages), false, true, Action{Kind: NextPage})
		statusRight = next.Min.X - gap
	}

	status := image.Rect(clock.Max.X, 0, statusRight, headerHeight-2)
	msg, col := "Home Assistant", darkGray
	if !m.Connected {
		msg, col = m.Status, black
		if msg == "" {
			msg = "Non connesso"
		}
	}
	text(img, face(!m.Connected, 24), msg, status, headerHeight/2, center, col)
	f.Regions = append(f.Regions, Region{Rect: status})
	fill(img, image.Rect(0, headerHeight-2, w, headerHeight), black)
}

// button draws a tappable box. A disabled button is greyed out and inert.
func button(f *Frame, r image.Rectangle, label string, inverted, enabled bool, a Action) {
	bg, fg, edge := white, black, black
	if inverted {
		bg, fg = black, white
	}
	if !enabled {
		fg, edge, a = midGray, midGray, Action{}
	}
	fill(f.Image, r, bg)
	border(f.Image, r, 3, edge)
	size := 28
	if len([]rune(label)) <= 1 {
		size = 48
	}
	inner := r.Inset(6)
	for size > 18 && measure(face(true, size), label) > inner.Dx() {
		size -= 2
	}
	text(f.Image, face(true, size), label, inner, (r.Min.Y+r.Max.Y)/2, center, fg)
	f.Regions = append(f.Regions, Region{Rect: r, Action: a})
}

func drawSensor(f *Frame, b block, name string, st ha.State, available bool) {
	r := b.rect
	border(f.Image, r, 1, midGray)
	value := "--"
	if available {
		value = st.State
		if v, err := strconv.ParseFloat(st.State, 64); err == nil {
			value = number(v)
		}
		unit := st.String("unit_of_measurement")
		if strings.HasPrefix(unit, "°") {
			unit = "°"
		}
		value += unit
	}
	inner := r.Inset(8)
	text(f.Image, face(true, 44), value, inner, r.Min.Y+36, center, black)
	small := face(false, 20)
	for i, line := range wrap(small, name, inner.Dx()) {
		text(f.Image, small, line, inner, r.Min.Y+74+i*22, center, darkGray)
	}
	f.Regions = append(f.Regions, Region{Rect: r})
}

func drawLight(f *Frame, b block, name string, st ha.State, available bool) {
	r := b.rect
	on := available && st.State == "on"
	bg, fg := white, black
	action := Action{Kind: Toggle, Entity: b.item.Entity}
	if on {
		bg, fg = black, white
	}
	fill(f.Image, r, bg)
	if available {
		border(f.Image, r, 3, black)
	} else {
		border(f.Image, r, 3, midGray)
		fg, action = midGray, Action{}
	}

	state := "spenta"
	switch {
	case !available:
		state = "non disponibile"
	case on:
		state = "accesa"
	}
	inner := r.Inset(10)
	big := face(true, 26)
	lines := wrap(big, name, inner.Dx())
	y := r.Min.Y + 38 - (len(lines)-1)*15
	for i, line := range lines {
		text(f.Image, big, line, inner, y+i*30, center, fg)
	}
	text(f.Image, face(false, 20), state, inner, r.Max.Y-22, center, fg)
	f.Regions = append(f.Regions, Region{Rect: r, Action: action})
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
	r := b.rect
	info := image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+infoHeight)
	fill(f.Image, image.Rect(info.Min.X, info.Min.Y, info.Max.X, info.Min.Y+3), black)

	inner := info.Inset(8)
	target, hasTarget := st.Float("temperature")
	if v, ok := m.Pending[b.item.Entity]; ok {
		target, hasTarget = v, true
	}
	targetText := "--"
	if available && hasTarget {
		targetText = number(target) + "°"
	}
	split := inner.Max.X - 150
	nameBox := image.Rect(inner.Min.X, inner.Min.Y, split, inner.Max.Y)
	text(f.Image, face(true, 30), name, nameBox, info.Min.Y+26, left, black)

	sub := "non disponibile"
	if available {
		sub = modeName(st.State)
		if cur, ok := st.Float("current_temperature"); ok {
			sub += " · ora " + number(cur) + "°"
		}
	}
	text(f.Image, face(false, 22), sub, nameBox, info.Min.Y+58, left, darkGray)
	text(f.Image, face(true, 48), targetText, image.Rect(split, inner.Min.Y, inner.Max.X, inner.Max.Y), info.Min.Y+infoHeight/2, right, black)
	f.Regions = append(f.Regions, Region{Rect: info})

	on := available && st.State != "off"
	width := (r.Dx() - 3*gap) / 4
	at := func(i int) image.Rectangle {
		x := r.Min.X + i*(width+gap)
		return image.Rect(x, info.Max.Y, x+width, r.Max.Y)
	}
	e := b.item.Entity
	button(f, at(0), "-", false, available && hasTarget, Action{TempDown, e})
	button(f, at(1), "+", false, available && hasTarget, Action{TempUp, e})
	mode := "Modo"
	if on {
		mode = modeName(st.State) + " ›"
	}
	button(f, at(2), mode, false, on, Action{Mode, e})
	power := "Accendi"
	if on {
		power = "Spegni"
	}
	button(f, at(3), power, on, available, Action{Power, e})
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
