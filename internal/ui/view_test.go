package ui

import (
	"image"
	"slices"
	"testing"

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
