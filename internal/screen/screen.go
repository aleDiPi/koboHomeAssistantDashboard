// Package screen puts grayscale images on the e-ink panel.
package screen

import "image"

// Mode selects how the e-ink panel updates.
type Mode int

const (
	Fast    Mode = iota // black/white only, quickest: touch feedback
	Quality             // 16 grays, no flash: content changes
	Flash               // full flashing refresh: removes ghosting
)

type Display interface {
	// Size is the logical size, after orientation.
	Size() (w, h int)
	// Show copies the area r of img to the panel and refreshes it.
	Show(img *image.Gray, r image.Rectangle, mode Mode) error
	Close() error
}

// Rotate maps a rectangle of a w×h logical canvas onto a panel rotated
// clockwise by deg degrees.
func Rotate(r image.Rectangle, w, h, deg int) image.Rectangle {
	switch deg {
	case 90:
		return image.Rect(h-r.Max.Y, r.Min.X, h-r.Min.Y, r.Max.X)
	case 180:
		return image.Rect(w-r.Max.X, h-r.Max.Y, w-r.Min.X, h-r.Min.Y)
	case 270:
		return image.Rect(r.Min.Y, w-r.Max.X, r.Max.Y, w-r.Min.X)
	}
	return r
}

// RotatePoint maps a pixel the same way Rotate maps rectangles.
func RotatePoint(p image.Point, w, h, deg int) image.Point {
	switch deg {
	case 90:
		return image.Pt(h-1-p.Y, p.X)
	case 180:
		return image.Pt(w-1-p.X, h-1-p.Y)
	case 270:
		return image.Pt(p.Y, w-1-p.X)
	}
	return p
}

// Rotation returns how many degrees the logical canvas must be turned to land
// on a panel of fbW×fbH pixels.
func Rotation(orientation string, fbW, fbH int) int {
	landscapeFB := fbW > fbH
	wantLandscape := orientation == "landscape" || orientation == "landscape_inverted"
	// On the Glo (portrait panel), "landscape" means the device turned so that
	// the USB port, at the bottom in portrait, is on the right.
	deg := 0
	if landscapeFB != wantLandscape {
		deg = 90
	}
	if orientation == "portrait_inverted" || orientation == "landscape_inverted" {
		deg = (deg + 180) % 360
	}
	return deg
}
