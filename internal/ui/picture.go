package ui

import (
	"image"
	_ "image/jpeg" // formats accepted for the standby picture
	_ "image/png"
	"os"

	"golang.org/x/image/draw"
)

// maxPicture guards the little memory of the device.
const maxPicture = 20_000_000 // pixels

// LoadPicture reads a picture and prepares it for a w×h e-ink panel: scaled to
// fit, in gray, with the contrast stretched and dithered to 16 levels.
func LoadPicture(path string, w, h int) (*image.Gray, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return nil, err
	}
	if cfg.Width*cfg.Height > maxPicture {
		return nil, image.ErrFormat
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return prepare(src, w, h), nil
}

func prepare(src image.Image, w, h int) *image.Gray {
	b := src.Bounds()
	// The whole picture, as large as it fits: artworks are not cropped.
	pw, ph := w, b.Dy()*w/b.Dx()
	if ph > h {
		pw, ph = b.Dx()*h/b.Dy(), h
	}
	pic := image.NewGray(image.Rect(0, 0, max(pw, 1), max(ph, 1)))
	draw.CatmullRom.Scale(pic, pic.Rect, src, b, draw.Src, nil)
	stretch(pic)
	dither(pic)
	return pic
}

// stretch spreads the grays over the whole range, ignoring the extreme 1%:
// old prints on yellowed paper look washed out otherwise.
func stretch(img *image.Gray) {
	var histogram [256]int
	for _, p := range img.Pix {
		histogram[p]++
	}
	limit := len(img.Pix) / 100
	low, high := 0, 255
	for n := 0; low < 255; low++ {
		if n += histogram[low]; n > limit {
			break
		}
	}
	for n := 0; high > 0; high-- {
		if n += histogram[high]; n > limit {
			break
		}
	}
	if high-low < 32 {
		return
	}
	for i, p := range img.Pix {
		img.Pix[i] = uint8(max(0, min(255, (int(p)-low)*255/(high-low))))
	}
}

// dither reduces to the 16 grays of the panel, diffusing the error
// (Floyd–Steinberg) so that gradients do not turn into bands.
func dither(img *image.Gray) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	current, next := make([]int, w+2), make([]int, w+2)
	for y := 0; y < h; y++ {
		row := img.Pix[y*img.Stride : y*img.Stride+w]
		for x := range row {
			v := max(0, min(255, int(row[x])+current[x+1]/16))
			q := (v + 8) / 17 * 17
			row[x] = uint8(q)
			e := v - q
			current[x+2] += e * 7
			next[x] += e * 3
			next[x+1] += e * 5
			next[x+2] += e
		}
		current, next = next, current
		clear(next)
	}
}
