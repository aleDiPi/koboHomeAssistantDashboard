package ui

import (
	"fmt"
	"image"
	"strconv"
	"time"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/ha"
)

var longMonths = [...]string{"gennaio", "febbraio", "marzo", "aprile", "maggio", "giugno",
	"luglio", "agosto", "settembre", "ottobre", "novembre", "dicembre"}

// day identifies a calendar date regardless of time zone.
type day struct {
	year  int
	month time.Month
	day   int
}

func dayOf(t time.Time) day {
	y, m, d := t.Date()
	return day{y, m, d}
}

func (d day) after(o day) bool {
	if d.year != o.year {
		return d.year > o.year
	}
	if d.month != o.month {
		return d.month > o.month
	}
	return d.day > o.day
}

// span returns the first and last date an event touches, as seen in loc.
func span(e ha.Event, loc *time.Location) (first, last time.Time) {
	if e.AllDay {
		y, m, d := e.Start.Date()
		first = time.Date(y, m, d, 12, 0, 0, 0, loc)
		y, m, d = e.End.Date()
		last = time.Date(y, m, d-1, 12, 0, 0, 0, loc)
	} else {
		first = e.Start.In(loc)
		// An event ending at midnight does not occupy the following day.
		last = e.End.Add(-time.Second).In(loc)
	}
	if last.Before(first) {
		last = first
	}
	return first, last
}

// drawCalendar fills area with the month grid and the upcoming events.
func drawCalendar(f *Frame, m Model, area image.Rectangle) {
	monthW := area.Dx() * 42 / 100
	month := image.Rect(area.Min.X, area.Min.Y, area.Min.X+monthW, area.Max.Y)
	list := image.Rect(month.Max.X+gap, area.Min.Y, area.Max.X, area.Max.Y)
	if area.Dx() < area.Dy() {
		// Portrait: one above the other.
		month = image.Rect(area.Min.X, area.Min.Y, area.Max.X, area.Min.Y+area.Dy()/2)
		list = image.Rect(area.Min.X, month.Max.Y+gap, area.Max.X, area.Max.Y)
	}
	drawMonth(f.Image, m, month)
	drawAgenda(f.Image, m, list)
	f.region(month, radius, Action{})
	f.region(list, radius, Action{})
}

func drawMonth(img *image.Gray, m Model, r image.Rectangle) {
	card(img, r, radius, 2, white, midGray)
	inner := r.Inset(12)
	title := fmt.Sprintf("%s %d", longMonths[m.Now.Month()-1], m.Now.Year())
	text(img, face(true, 26), title, inner, r.Min.Y+28, center, black)

	busy := map[day]bool{}
	for _, e := range m.Events {
		first, last := span(e, m.Now.Location())
		// A period of weeks (a heating season) would mark every day.
		if last.Sub(first) > 7*24*time.Hour {
			continue
		}
		for d, n := first, 0; n < 62 && (d.Before(last) || dayOf(d) == dayOf(last)); d, n = d.AddDate(0, 0, 1), n+1 {
			busy[dayOf(d)] = true
		}
	}

	cellW := inner.Dx() / 7
	top := r.Min.Y + 54
	for i, label := range []string{"L", "M", "M", "G", "V", "S", "D"} {
		cell := image.Rect(inner.Min.X+i*cellW, top, inner.Min.X+(i+1)*cellW, top+24)
		text(img, face(false, 18), label, cell, top+12, center, darkGray)
	}
	top += 30

	first := time.Date(m.Now.Year(), m.Now.Month(), 1, 12, 0, 0, 0, m.Now.Location())
	offset := (int(first.Weekday()) + 6) % 7 // weeks start on Monday
	days := first.AddDate(0, 1, -1).Day()
	rows := (offset + days + 6) / 7
	cellH := min((r.Max.Y-8-top)/rows, 52)
	for n := 1; n <= days; n++ {
		i := offset + n - 1
		x, y := inner.Min.X+(i%7)*cellW, top+(i/7)*cellH
		cell := image.Rect(x, y, x+cellW, y+cellH)
		cx, cy := float64(x)+float64(cellW)/2, float64(y)+float64(cellH)/2-3
		date := first.AddDate(0, 0, n-1)
		fg, dot, font := black, black, face(false, 21)
		if date.Weekday() == time.Sunday {
			fg = darkGray
		}
		if n == m.Now.Day() {
			radius := float64(min(cellW, cellH))/2 - 1
			paint(img, cell, circle(cx, cy+2, radius), black)
			fg, dot, font = white, white, face(true, 21)
		}
		text(img, font, strconv.Itoa(n), cell, int(cy), center, fg)
		if busy[dayOf(date)] {
			paint(img, cell, circle(cx, cy+14, 3), dot)
		}
	}
}

// when describes the start of an event relative to now.
func when(e ha.Event, now time.Time) string {
	first, last := span(e, now.Location())
	today, tomorrow := dayOf(now), dayOf(now.AddDate(0, 0, 1))
	if !e.AllDay && !e.Start.After(now) {
		end := e.End.In(now.Location())
		if dayOf(end) == today {
			return "adesso, fino alle " + end.Format("15:04")
		}
		return fmt.Sprintf("adesso, fino al %d %s", end.Day(), months[end.Month()-1])
	}
	if e.AllDay && !dayOf(first).after(today) && dayOf(last).after(today) {
		return fmt.Sprintf("in corso, fino al %d %s", last.Day(), months[last.Month()-1])
	}
	label := fmt.Sprintf("%s %d %s", weekdays[first.Weekday()], first.Day(), months[first.Month()-1])
	switch {
	case dayOf(first) == today, e.AllDay && first.Before(now) && !last.Before(now):
		label = "oggi"
	case dayOf(first) == tomorrow:
		label = "domani"
	}
	if !e.AllDay {
		label += " alle " + first.Format("15:04")
	}
	return label
}

// upcoming drops the events that are already over.
func upcoming(events []ha.Event, now time.Time) []ha.Event {
	var out []ha.Event
	for _, e := range events {
		_, last := span(e, now.Location())
		if e.AllDay && (dayOf(last) == dayOf(now) || last.After(now)) || !e.AllDay && e.End.After(now) {
			out = append(out, e)
		}
	}
	return out
}

func drawAgenda(img *image.Gray, m Model, r image.Rectangle) {
	card(img, r, radius, 2, white, midGray)
	inner := r.Inset(18)
	text(img, face(true, 26), "In programma", inner, r.Min.Y+28, left, black)

	events := upcoming(m.Events, m.Now)
	if len(events) == 0 {
		msg := "Nessun evento in programma"
		if len(m.Config.Calendars) == 0 {
			msg = "Nessun calendario configurato"
		}
		text(img, face(false, 22), msg, inner, r.Min.Y+80, left, darkGray)
		return
	}
	names := map[string]string{}
	for _, c := range m.Config.Calendars {
		names[c.Entity] = c.Name
	}
	const rowH = 60
	y := r.Min.Y + 56
	for _, e := range events {
		if y+rowH > r.Max.Y-4 {
			break
		}
		line := when(e, m.Now)
		if name := names[e.Calendar]; name != "" {
			line += " · " + name
		}
		fill(img, image.Rect(inner.Min.X, y+8, inner.Min.X+5, y+rowH-8), black)
		box := image.Rect(inner.Min.X+16, y, inner.Max.X, y+rowH)
		text(img, face(true, 23), e.Summary, box, y+19, left, black)
		text(img, face(false, 18), line, box, y+43, left, darkGray)
		y += rowH
	}
}
