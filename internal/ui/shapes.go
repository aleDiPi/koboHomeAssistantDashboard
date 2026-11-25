package ui

// Anti-aliased shapes, described by signed distance functions: negative inside.

import (
	"image"
	"math"
)

type sdf func(x, y float64) float64

// paint blends gray g over the pixels of area covered by the shape.
func paint(img *image.Gray, area image.Rectangle, d sdf, g uint8) {
	area = area.Intersect(img.Rect)
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			c := 0.5 - d(float64(x)+0.5, float64(y)+0.5)
			if c <= 0 {
				continue
			}
			i := img.PixOffset(x, y)
			if c >= 1 {
				img.Pix[i] = g
				continue
			}
			img.Pix[i] = uint8(float64(img.Pix[i])*(1-c) + float64(g)*c + 0.5)
		}
	}
}

func roundRect(r image.Rectangle, radius float64) sdf {
	cx, cy := float64(r.Min.X+r.Max.X)/2, float64(r.Min.Y+r.Max.Y)/2
	hw, hh := float64(r.Dx())/2, float64(r.Dy())/2
	radius = math.Min(radius, math.Min(hw, hh))
	return func(x, y float64) float64 {
		dx, dy := math.Abs(x-cx)-(hw-radius), math.Abs(y-cy)-(hh-radius)
		return math.Hypot(math.Max(dx, 0), math.Max(dy, 0)) + math.Min(math.Max(dx, dy), 0) - radius
	}
}

func circle(cx, cy, radius float64) sdf {
	return func(x, y float64) float64 { return math.Hypot(x-cx, y-cy) - radius }
}

// segment is a line with round ends.
func segment(ax, ay, bx, by, width float64) sdf {
	vx, vy := bx-ax, by-ay
	length := vx*vx + vy*vy
	return func(x, y float64) float64 {
		t := 0.0
		if length > 0 {
			t = math.Max(0, math.Min(1, ((x-ax)*vx+(y-ay)*vy)/length))
		}
		return math.Hypot(x-ax-t*vx, y-ay-t*vy) - width/2
	}
}

// outline keeps a band of the given width just inside the shape's edge.
func outline(d sdf, width float64) sdf {
	return func(x, y float64) float64 { return math.Abs(d(x, y)+width/2) - width/2 }
}

// card draws a rounded box with a border.
func card(img *image.Gray, r image.Rectangle, radius, width float64, bg, edge uint8) {
	d := roundRect(r, radius)
	paint(img, r, d, bg)
	if width > 0 {
		paint(img, r, outline(d, width), edge)
	}
}

func line(img *image.Gray, ax, ay, bx, by, width float64, g uint8) {
	area := image.Rect(
		int(math.Min(ax, bx)-width)-1, int(math.Min(ay, by)-width)-1,
		int(math.Max(ax, bx)+width)+2, int(math.Max(ay, by)+width)+2)
	paint(img, area, segment(ax, ay, bx, by, width), g)
}

// bulb draws a light bulb centred on (cx, cy); a lit one is solid and shines.
func bulb(img *image.Gray, cx, cy float64, on bool, g uint8) {
	area := image.Rect(int(cx)-32, int(cy)-34, int(cx)+33, int(cy)+34)
	glass := circle(cx, cy-5, 13)
	if on {
		paint(img, area, glass, g)
		for _, deg := range []float64{180, 225, 270, 315, 360} {
			s, c := math.Sincos(deg * math.Pi / 180)
			line(img, cx+19*c, cy-5+19*s, cx+26*c, cy-5+26*s, 3, g)
		}
	} else {
		paint(img, area, outline(glass, 3), g)
	}
	line(img, cx-6, cy+12, cx+6, cy+12, 3, g)
	line(img, cx-5, cy+18, cx+5, cy+18, 3, g)
}

// powerSymbol is the usual broken ring with a bar.
func powerSymbol(img *image.Gray, cx, cy float64, g uint8) {
	area := image.Rect(int(cx)-22, int(cy)-22, int(cx)+23, int(cy)+23)
	ring := outline(circle(cx, cy, 17), 5)
	paint(img, area, func(x, y float64) float64 {
		// leave a gap at the top for the bar
		if y < cy && math.Abs(x-cx) < 8 {
			return 1
		}
		return ring(x, y)
	}, g)
	line(img, cx, cy-19, cx, cy-4, 5, g)
}
