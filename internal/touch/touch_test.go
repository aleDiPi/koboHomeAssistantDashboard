package touch

import (
	"image"
	"testing"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/screen"
)

type ev struct {
	typ, code uint16
	val       int32
}

func taps(events []ev) []image.Point {
	var d Decoder
	var out []image.Point
	for _, e := range events {
		if p, ok := d.Feed(e.typ, e.code, e.val); ok {
			out = append(out, p)
		}
	}
	return out
}

func TestDecoderDialects(t *testing.T) {
	syn := ev{evSyn, synReport, 0}
	for name, events := range map[string][]ev{
		"single touch": {
			{evAbs, absX, 100}, {evAbs, absY, 200}, {evAbs, absPressure, 1024}, syn,
			{evAbs, absX, 100}, {evAbs, absY, 200}, {evAbs, absPressure, 0}, syn,
		},
		"multitouch protocol B": {
			{evAbs, absMTTrackingID, 3}, {evAbs, absMTPositionX, 100}, {evAbs, absMTPositionY, 200}, syn,
			{evAbs, absMTTrackingID, -1}, syn,
		},
		"btn_touch": {
			{evKey, btnTouch, 1}, {evAbs, absX, 100}, {evAbs, absY, 200}, syn,
			{evKey, btnTouch, 0}, syn,
		},
	} {
		got := taps(events)
		if len(got) != 1 || got[0] != image.Pt(100, 200) {
			t.Errorf("%s: got %v", name, got)
		}
	}
}

// A touch on a given spot of the glass must land on the pixel that was drawn
// there, whatever the orientation.
func TestTransformInvertsRotation(t *testing.T) {
	const w, h = 758, 1024
	for _, deg := range []int{0, 90, 180, 270} {
		lw, lh := w, h
		if deg == 90 || deg == 270 {
			lw, lh = h, w
		}
		want := image.Pt(30, 70)
		panel := screen.RotatePoint(want, lw, lh, deg)
		tr := Transform{Width: w, Height: h, Rotation: deg}
		if got := tr.Apply(panel); got != want {
			t.Errorf("deg %d: got %v, want %v", deg, got, want)
		}
	}
}
