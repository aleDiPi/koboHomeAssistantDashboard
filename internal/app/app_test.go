package app

import (
	"image"
	"testing"
	"time"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/config"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/ha"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/screen"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/ui"
)

type shown struct {
	r    image.Rectangle
	mode screen.Mode
}

type fakeDisplay struct{ log []shown }

func (f *fakeDisplay) Size() (int, int) { return 1024, 758 }
func (f *fakeDisplay) Close() error     { return nil }
func (f *fakeDisplay) Show(_ *image.Gray, r image.Rectangle, m screen.Mode) error {
	f.log = append(f.log, shown{r, m})
	return nil
}

func find(t *testing.T, f ui.Frame, k ui.Kind) ui.Region {
	t.Helper()
	for _, r := range f.Regions {
		if r.Action.Kind == k {
			return r
		}
	}
	t.Fatalf("no region with action %d", k)
	return ui.Region{}
}

func centre(r image.Rectangle) image.Point { return r.Min.Add(r.Max).Div(2) }

func testApp(view ui.View) (*App, *Demo, *fakeDisplay) {
	cfg := config.Default()
	cfg.Lights = []config.Item{{Entity: "light.a", Name: "A"}, {Entity: "light.b", Name: "B"}}
	cfg.Climate = []config.Item{{Entity: "climate.c", Name: "C"}}
	demo, disp := NewDemo(cfg), &fakeDisplay{}
	a := New(cfg, demo, disp, time.UTC)
	a.view = view
	a.render(true)
	return a, demo, disp
}

// A tap on a light must refresh only that tile.
func TestTapRefreshesOnlyTheTouchedTile(t *testing.T) {
	a, demo, disp := testApp(ui.Lights)
	tile := find(t, a.frame, ui.Toggle)
	before, _, _ := demo.Snapshot()

	disp.log = nil
	a.tap(centre(tile.Rect))
	a.render(false)

	after, _, _ := demo.Snapshot()
	if before[tile.Action.Entity].State == after[tile.Action.Entity].State {
		t.Fatal("light was not toggled")
	}
	// The tile twice (feedback, then new state) and the "lights on" counter of
	// the navigation bar; nothing else.
	if len(disp.log) != 3 {
		t.Fatalf("refreshes: %v", disp.log)
	}
	for i, r := range disp.log {
		inNav := i == 2 && r.r.Min.Y >= 758-navigation
		if r.mode == screen.Flash || r.r != tile.Rect && !inNav {
			t.Errorf("unexpected refresh %v", r)
		}
	}
}

// Height of the bottom bar, margin included.
const navigation = 80

func TestTemperatureStepsAreSentOnce(t *testing.T) {
	a, demo, _ := testApp(ui.Climate)
	up := find(t, a.frame, ui.TempUp)
	a.tap(centre(up.Rect))
	a.render(false)
	a.tap(centre(up.Rect))

	st, _, _ := demo.Snapshot()
	if v, _ := st["climate.c"].Float("temperature"); v != 22 {
		t.Fatalf("sent too early: %v", v)
	}
	a.flushTemps(time.Now().Add(2 * time.Second))
	st, _, _ = demo.Snapshot()
	if v, _ := st["climate.c"].Float("temperature"); v != 24 {
		t.Fatalf("temperature = %v, want 24", v)
	}
}

func TestExitNeedsTwoTaps(t *testing.T) {
	a, _, _ := testApp(ui.Home)
	exit := find(t, a.frame, ui.Exit)
	if done, _ := a.tap(centre(exit.Rect)); done {
		t.Fatal("exited on the first tap")
	}
	a.render(false)
	if done, _ := a.tap(centre(exit.Rect)); !done {
		t.Fatal("did not exit on the second tap")
	}
}

func TestLightLevel(t *testing.T) {
	c := config.Default().Frontlight
	c.NightWhenOff = []string{"light.a", "light.b"}
	states := func(sun, a, b string) map[string]ha.State {
		return map[string]ha.State{"sun.sun": {State: sun}, "light.a": {State: a}, "light.b": {State: b}}
	}
	for name, tc := range map[string]struct {
		states map[string]ha.State
		since  time.Duration
		want   int
	}{
		"day":                  {states("above_horizon", "off", "off"), time.Second, 0},
		"dark room":            {states("below_horizon", "off", "off"), time.Hour, c.NightLevel},
		"room lit":             {states("below_horizon", "off", "on"), time.Hour, 0},
		"touched after sunset": {states("below_horizon", "on", "on"), time.Second, c.TouchLevel},
	} {
		if got := lightLevel(c, tc.states, tc.since); got != tc.want {
			t.Errorf("%s: %d, want %d", name, got, tc.want)
		}
	}
}

func TestNextMode(t *testing.T) {
	modes := []string{"off", "cool", "heat"}
	for current, want := range map[string]string{"cool": "heat", "heat": "cool", "off": "cool"} {
		if got := nextMode(modes, current); got != want {
			t.Errorf("after %s: %s, want %s", current, got, want)
		}
	}
}

func TestNavigationAndReturnHome(t *testing.T) {
	a, _, disp := testApp(ui.Home)
	var lights ui.Region
	for _, r := range a.frame.Regions {
		if r.Action.Kind == ui.Show && r.Action.View == ui.Lights {
			lights = r
		}
	}
	a.lastTouch = time.Now()
	a.tap(centre(lights.Rect))
	disp.log = nil
	a.render(false)
	if a.frame.View != ui.Lights || len(disp.log) != 1 || disp.log[0].mode != screen.Flash {
		t.Fatalf("view %d, refreshes %v", a.frame.View, disp.log)
	}
	if a.returnHome(time.Now()) {
		t.Fatal("went home while in use")
	}
	if !a.returnHome(time.Now().Add(time.Hour)) || a.view != ui.Home {
		t.Fatal("did not go home when idle")
	}
}

// A window must not open on a single accidental touch.
func TestOpeningAWindowNeedsTwoTaps(t *testing.T) {
	cfg := config.Default()
	cfg.Windows = []config.Item{{Entity: "cover.a", Name: "A"}, {Entity: "cover.b", Name: "B"}}
	demo := NewDemo(cfg)
	a := New(cfg, demo, &fakeDisplay{}, time.UTC)
	a.view = ui.Climate
	a.render(true)
	var open ui.Region
	for _, r := range a.frame.Regions {
		if r.Action.Kind == ui.OpenCover && r.Action.Entity == "cover.b" {
			open = r
		}
	}
	state := func() string {
		s, _, _ := demo.Snapshot()
		return s["cover.b"].State
	}
	a.tap(centre(open.Rect))
	a.render(false)
	if state() != "closed" {
		t.Fatal("opened on the first tap")
	}
	a.tap(centre(open.Rect))
	if state() != "open" {
		t.Fatal("did not open on the second tap")
	}
}
