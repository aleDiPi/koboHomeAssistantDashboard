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
		Layout: config.Layout{Notifications: []config.Item{
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

func TestIcons(t *testing.T) {
	for _, tc := range []struct {
		name string
		on   bool
		want string
	}{
		{"lightbulb", true, "lightbulb"},
		{"lightbulb", false, "lightbulb-outline"},
		{"track-light", false, "track-light"}, // no outline variant
	} {
		if got := stateIcon(tc.name, tc.on); got != tc.want {
			t.Errorf("%s on=%v: %s", tc.name, tc.on, got)
		}
	}
	if got := iconName("", "mdi:does-not-exist", "mdi:sofa"); got != "sofa" {
		t.Errorf("got %s", got)
	}
	img := image.NewGray(image.Rect(0, 0, 100, 100))
	icon(img, "lightbulb", 60, 50, 50, white)
	var ink image.Rectangle
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			if img.GrayAt(x, y).Y > 128 {
				ink = ink.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if c := ink.Min.Add(ink.Max).Div(2); ink.Dy() < 30 || c.X < 44 || c.X > 56 || c.Y < 42 || c.Y > 58 {
		t.Errorf("icon drawn at %v", ink)
	}
}

// A page mixing every kind of entity must keep all of them on screen.
func TestFlowKeepsEverythingInside(t *testing.T) {
	items := func(n int) []config.Item { return make([]config.Item, n) }
	p := config.Page{Sensors: items(5), Lights: items(9), Climate: items(3), Windows: items(4), Calendars: items(1)}
	area := body(1024, 758)
	blocks, sheets := flow(p, area, true)
	if len(blocks) != 5+9+3+4+1 || sheets < 2 {
		t.Fatalf("%d blocks on %d sheets", len(blocks), sheets)
	}
	for i, b := range blocks {
		if !b.rect.In(area) {
			t.Errorf("block %d outside: %v", i, b.rect)
		}
		for _, o := range blocks[:i] {
			if o.page == b.page && o.rect.Overlaps(b.rect) {
				t.Errorf("overlap %v %v", o.rect, b.rect)
			}
		}
	}
}

func TestCountdown(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	m := Model{Now: now, States: map[string]ha.State{"input_number.min": {State: "10.0"}}}
	it := config.Item{Entity: "script.air", Timer: "input_number.min"}
	running := ha.State{State: "on", Attr: map[string]any{"last_triggered": "2026-11-25T11:57:28.5+00:00"}}
	leftover, total, ok := remaining(m, it, running)
	if !ok || total != 10*time.Minute || clockLeft(leftover) != "7:30" {
		t.Errorf("left %v of %v (%v)", leftover, total, ok)
	}
	if _, _, ok := remaining(m, it, ha.State{State: "off", Attr: running.Attr}); ok {
		t.Error("a stopped script has no countdown")
	}
	if d, ok := duration(m, "2.5"); !ok || d != 150*time.Second {
		t.Errorf("fixed minutes: %v", d)
	}
}
