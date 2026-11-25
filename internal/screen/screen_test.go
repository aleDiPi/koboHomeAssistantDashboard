package screen

import (
	"image"
	"testing"
)

func TestRotateRectMatchesPoints(t *testing.T) {
	const w, h = 7, 11
	r := image.Rect(1, 2, 4, 9)
	for _, deg := range []int{0, 90, 180, 270} {
		got := Rotate(r, w, h, deg)
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				if p := RotatePoint(image.Pt(x, y), w, h, deg); !p.In(got) {
					t.Fatalf("deg %d: point %v maps to %v outside %v", deg, image.Pt(x, y), p, got)
				}
			}
		}
		if got.Dx()*got.Dy() != r.Dx()*r.Dy() {
			t.Fatalf("deg %d: area changed", deg)
		}
	}
}

// With the USB port on the right the device is turned counterclockwise: the
// viewer's top-left corner is the panel's top-right one.
func TestLandscapeHasUSBOnTheRight(t *testing.T) {
	deg := Rotation("landscape", 758, 1024)
	if got := RotatePoint(image.Pt(0, 0), 1024, 758, deg); got != image.Pt(757, 0) {
		t.Errorf("top-left lands on %v", got)
	}
}
