// Package touch turns Linux input events into taps.
package touch

import "image"

// Linux input event types and codes.
const (
	evSyn = 0x00
	evKey = 0x01
	evAbs = 0x03

	synReport = 0x00
	btnTouch  = 0x14a

	absX            = 0x00
	absY            = 0x01
	absPressure     = 0x18
	absMTTouchMajor = 0x30
	absMTPositionX  = 0x35
	absMTPositionY  = 0x36
	absMTTrackingID = 0x39
	absMTPressure   = 0x3a
)

// Decoder accepts the single-touch and multitouch dialects used by the
// different Kobo touch panels. A tap is reported when the finger lifts.
type Decoder struct {
	x, y    int
	hasPos  bool
	down    bool
	pending bool // contact state seen in the current packet
}

// Feed processes one event and returns the raw tap position, if any.
func (d *Decoder) Feed(typ, code uint16, value int32) (image.Point, bool) {
	switch typ {
	case evAbs:
		switch code {
		case absX, absMTPositionX:
			d.x, d.hasPos = int(value), true
		case absY, absMTPositionY:
			d.y, d.hasPos = int(value), true
		case absPressure, absMTPressure, absMTTouchMajor:
			d.pending = value > 0
		case absMTTrackingID:
			d.pending = value >= 0
		}
	case evKey:
		if code == btnTouch {
			d.pending = value != 0
		}
	case evSyn:
		if code != synReport {
			break
		}
		wasDown := d.down
		d.down = d.pending
		if wasDown && !d.down && d.hasPos {
			d.hasPos = false
			return image.Pt(d.x, d.y), true
		}
	}
	return image.Point{}, false
}

// Transform converts raw panel coordinates into logical screen coordinates.
// First the panel is aligned to the portrait screen (swap, mirror), then the
// user's orientation is applied.
type Transform struct {
	SwapXY, MirrorX, MirrorY bool
	// Size of the screen the swapped coordinates refer to (portrait).
	Width, Height int
	// Clockwise rotation from the logical canvas to that portrait screen.
	Rotation int
}

func (t Transform) Apply(p image.Point) image.Point {
	if t.SwapXY {
		p.X, p.Y = p.Y, p.X
	}
	if t.MirrorX {
		p.X = t.Width - 1 - p.X
	}
	if t.MirrorY {
		p.Y = t.Height - 1 - p.Y
	}
	// Inverse of screen.RotatePoint.
	switch t.Rotation {
	case 90:
		return image.Pt(p.Y, t.Width-1-p.X)
	case 180:
		return image.Pt(t.Width-1-p.X, t.Height-1-p.Y)
	case 270:
		return image.Pt(t.Height-1-p.Y, p.X)
	}
	return p
}
