package ui

import (
	"image"
	"image/color"
	"image/draw"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	black     uint8 = 0x00
	darkGray  uint8 = 0x55
	midGray   uint8 = 0xAA
	lightGray uint8 = 0xDD
	white     uint8 = 0xFF
)

type faceKey struct {
	bold bool
	size int
}

var (
	regular = mustParse(goregular.TTF)
	bold    = mustParse(gobold.TTF)
	faces   = map[faceKey]font.Face{} // used only by the render goroutine
)

func mustParse(ttf []byte) *opentype.Font {
	f, err := opentype.Parse(ttf)
	if err != nil {
		panic(err)
	}
	return f
}

// face returns a font whose size is expressed in pixels.
func face(isBold bool, size int) font.Face {
	k := faceKey{isBold, size}
	if f, ok := faces[k]; ok {
		return f
	}
	src := regular
	if isBold {
		src = bold
	}
	f, err := opentype.NewFace(src, &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err)
	}
	faces[k] = f
	return f
}

func fill(img *image.Gray, r image.Rectangle, g uint8) {
	draw.Draw(img, r, image.NewUniform(color.Gray{Y: g}), image.Point{}, draw.Src)
}

type align int

const (
	left align = iota
	center
	right
)

func measure(f font.Face, s string) int { return font.MeasureString(f, s).Ceil() }

// fit shortens s with an ellipsis until it is at most width pixels wide.
func fit(f font.Face, s string, width int) string {
	if measure(f, s) <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && measure(f, string(r)+"…") > width {
		r = r[:len(r)-1]
	}
	return strings.TrimRight(string(r), " ") + "…"
}

// wrap splits s into at most two lines of at most width pixels.
func wrap(f font.Face, s string, width int) []string {
	words := strings.Fields(s)
	first := ""
	for i, w := range words {
		next := strings.TrimSpace(first + " " + w)
		if measure(f, next) > width {
			if first == "" {
				break
			}
			return []string{first, fit(f, strings.Join(words[i:], " "), width)}
		}
		first = next
	}
	return []string{fit(f, s, width)}
}

// text writes one line centred vertically on cy, aligned inside r.
func text(img *image.Gray, f font.Face, s string, r image.Rectangle, cy int, a align, g uint8) {
	s = fit(f, s, r.Dx())
	x := r.Min.X
	switch a {
	case center:
		x += (r.Dx() - measure(f, s)) / 2
	case right:
		x = r.Max.X - measure(f, s)
	}
	m := f.Metrics()
	baseline := cy + (m.Ascent.Ceil()-m.Descent.Ceil())/2
	d := font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.Gray{Y: g}),
		Face: f,
		Dot:  fixed.P(x, baseline),
	}
	d.DrawString(s)
}
