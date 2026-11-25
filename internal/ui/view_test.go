package ui

import (
	"image"
	"slices"
	"testing"
	"time"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/config"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/ha"
)

func TestOnlyActiveNotices(t *testing.T) {
	m := Model{
		Config: config.Config{Notifications: []config.Item{
			{Entity: "binary_sensor.window", Name: "Window"},
			{Entity: "cover.roof", Name: "Roof"},
			{Entity: "binary_sensor.closed", Name: "Closed"},
			{Entity: "binary_sensor.gone", Name: "Gone"},
		}},
		States: map[string]ha.State{
			"binary_sensor.window": {State: "on"},
			"cover.roof":           {State: "open"},
			"binary_sensor.closed": {State: "off"},
		},
	}
	if got := notices(m); !slices.Equal(got, []string{"Window", "Roof"}) {
		t.Errorf("got %v", got)
	}
}

// Chips must never spill outside their area.
func TestChipsStayInside(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 600, headerHeight))
	for i := range img.Pix {
		img.Pix[i] = white
	}
	area := image.Rect(100, 0, 400, headerHeight)
	drawChips(img, area, []string{"A very long notification label that cannot fit", "Two", "Three", "Four"})
	for y := 0; y < headerHeight; y++ {
		for x := 0; x < 600; x++ {
			if !image.Pt(x, y).In(area) && img.GrayAt(x, y).Y != white {
				t.Fatalf("painted outside at %d,%d", x, y)
			}
		}
	}
}

func TestWhenAndUpcoming(t *testing.T) {
	loc := time.FixedZone("cet", 3600)
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, loc)
	at := func(d, h int) time.Time { return time.Date(2026, 11, d, h, 0, 0, 0, loc) }
	date := func(d int) time.Time { return time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC) }
	events := []ha.Event{
		{Summary: "over", Start: at(25, 8), End: at(25, 9)},
		{Summary: "running", Start: at(25, 11), End: at(25, 13)},
		{Summary: "tomorrow", Start: at(26, 9), End: at(26, 10)},
		{Summary: "today all day", Start: date(25), End: date(26), AllDay: true},
		{Summary: "yesterday all day", Start: date(24), End: date(25), AllDay: true},
	}
	var got []string
	for _, e := range upcoming(events, now) {
		got = append(got, e.Summary+": "+when(e, now))
	}
	want := []string{"running: adesso, fino alle 13:00", "tomorrow: domani alle 09:00", "today all day: oggi"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q", got)
	}
}
