package app

import (
	"image"
	"testing"
	"time"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/config"
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

func testApp() (*App, *Demo, *fakeDisplay) {
	cfg := config.Default()
	cfg.Lights = []config.Item{{Entity: "light.a", Name: "A"}, {Entity: "light.b", Name: "B"}}
	cfg.Climate = []config.Item{{Entity: "climate.c", Name: "C"}}
	demo, disp := NewDemo(cfg), &fakeDisplay{}
	a := New(cfg, demo, disp, time.UTC)
	a.render(true)
	return a, demo, disp
}

// A tap on a light must refresh only that tile.
func TestTapRefreshesOnlyTheTouchedTile(t *testing.T) {
	a, demo, disp := testApp()
	tile := find(t, a.frame, ui.Toggle)
	before, _, _ := demo.Snapshot()

	disp.log = nil
	a.tap(centre(tile.Rect))
	a.render(false)

	after, _, _ := demo.Snapshot()
	if before[tile.Action.Entity].State == after[tile.Action.Entity].State {
		t.Fatal("light was not toggled")
	}
	if len(disp.log) != 2 {
		t.Fatalf("refreshes: %v", disp.log)
	}
	for _, s := range disp.log {
		if s.r != tile.Rect || s.mode == screen.Flash {
			t.Errorf("unexpected refresh %v", s)
		}
	}
}

func TestTemperatureStepsAreSentOnce(t *testing.T) {
	a, demo, _ := testApp()
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
	a, _, _ := testApp()
	exit := find(t, a.frame, ui.Exit)
	if done, _ := a.tap(centre(exit.Rect)); done {
		t.Fatal("exited on the first tap")
	}
	a.render(false)
	if done, _ := a.tap(centre(exit.Rect)); !done {
		t.Fatal("did not exit on the second tap")
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
