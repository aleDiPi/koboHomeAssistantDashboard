package ui

import (
	_ "embed"
	"image"
	"image/color"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Material Design Icons, the set Home Assistant uses. See mdi/LICENSE.txt.
var (
	//go:embed mdi/mdi.ttf
	mdiFont []byte
	//go:embed mdi/codepoints.txt
	mdiNames string
)

var codepoints = sync.OnceValue(func() map[string]rune {
	m := map[string]rune{}
	for _, row := range strings.Split(mdiNames, "\n") {
		name, hex, ok := strings.Cut(strings.TrimSpace(row), " ")
		if !ok {
			continue
		}
		if v, err := strconv.ParseInt(hex, 16, 32); err == nil {
			m[name] = rune(v)
		}
	}
	return m
})

var (
	icons     = sync.OnceValue(func() *opentype.Font { return mustParse(mdiFont) })
	iconFaces = map[int]font.Face{} // used only by the render goroutine
)

func iconFace(size int) font.Face {
	if f, ok := iconFaces[size]; ok {
		return f
	}
	f, err := opentype.NewFace(icons(), &opentype.FaceOptions{Size: float64(size), DPI: 72})
	if err != nil {
		panic(err)
	}
	iconFaces[size] = f
	return f
}

// iconName picks the first known icon among the candidates, written with or
// without the "mdi:" prefix.
func iconName(candidates ...string) string {
	for _, c := range candidates {
		c = strings.TrimPrefix(c, "mdi:")
		if _, ok := codepoints()[c]; ok {
			return c
		}
	}
	return "help-circle-outline"
}

// stateIcon returns the solid icon for something on, and its outline variant,
// where one exists, for something off.
func stateIcon(name string, on bool) string {
	base := strings.TrimSuffix(name, "-outline")
	if on {
		return iconName(base, name)
	}
	return iconName(base+"-outline", name)
}

// icon draws the named icon, size pixels wide, centred on (cx, cy).
func icon(img *image.Gray, name string, size, cx, cy int, g uint8) {
	r, ok := codepoints()[iconName(name)]
	if !ok {
		return
	}
	f := iconFace(size)
	bounds, _, ok := f.GlyphBounds(r)
	if !ok {
		return
	}
	// Icons are designed on a square: centre the em box, not the ink, so that
	// variants of one icon line up.
	m := f.Metrics()
	advance, _ := f.GlyphAdvance(r)
	x := cx - advance.Round()/2
	y := cy + (m.Ascent.Round()-m.Descent.Round())/2
	if h := (bounds.Max.Y - bounds.Min.Y).Round(); h > size*2 {
		y = cy - (bounds.Min.Y+bounds.Max.Y).Round()/2
	}
	d := font.Drawer{Dst: img, Src: image.NewUniform(color.Gray{Y: g}), Face: f, Dot: fixed.P(x, y)}
	d.DrawString(string(r))
}
