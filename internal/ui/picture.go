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

// LoadPicture reads a picture and prepares it for a w×h e-ink panel: cropped
// and scaled to fill it, in gray, with the contrast stretched and dithered to 16 levels.
func LoadPicture(path string, w, h int, focus float64) (*image.Gray, error) {
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
	return prepare(src, w, h, focus), nil
}

// crop returns the largest part of b with the proportions of w×h. focus says
// what to keep of the excess: 0 the left (or top) side, 1 the right (bottom).
func crop(b image.Rectangle, w, h int, focus float64) image.Rectangle {
	cw, ch := b.Dx(), b.Dx()*h/w
	if ch > b.Dy() {
		cw, ch = b.Dy()*w/h, b.Dy()
	}
	x := b.Min.X + int(float64(b.Dx()-cw)*focus+0.5)
	y := b.Min.Y + int(float64(b.Dy()-ch)*focus+0.5)
	return image.Rect(x, y, x+cw, y+ch)
}

func prepare(src image.Image, w, h int, focus float64) *image.Gray {
	pic := image.NewGray(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(pic, pic.Rect, src, crop(src.Bounds(), w, h, focus), draw.Src, nil)
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
