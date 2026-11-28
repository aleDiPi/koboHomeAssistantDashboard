package app

import (
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/ui"
)

// listPictures returns path itself, or the pictures of a folder in name order.
func listPictures(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{path}, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		// Names starting with a dot are leftovers of other systems.
		if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") && (ext == ".jpg" || ext == ".jpeg" || ext == ".png") {
			out = append(out, filepath.Join(path, e.Name()))
		}
	}
	slices.Sort(out)
	return out, nil
}

// ofTheDay picks one picture per calendar day, in turn.
func ofTheDay(list []string, now time.Time) string {
	if len(list) == 0 {
		return ""
	}
	y, m, d := now.Date()
	days := time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400
	return list[int(days%int64(len(list)))]
}

// focusOf reads the crop position from a name such as "wave@0.12.jpg".
func focusOf(path string, fallback float64) float64 {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if _, at, ok := strings.Cut(name, "@"); ok {
		if v, err := strconv.ParseFloat(at, 64); err == nil && v >= 0 && v <= 1 {
			return v
		}
	}
	return fallback
}

// SetPictures sets the standby pictures: one file, or a folder whose pictures
// take turns day by day. Without any, a plain notice is shown.
func (a *App) SetPictures(path string) {
	list, err := listPictures(path)
	if err != nil {
		log.Printf("standby pictures: %v", err)
		return
	}
	a.pictures = list
	a.pickPicture()
}

// pickPicture loads the picture of today, if it is not the one already loaded.
func (a *App) pickPicture() {
	path := ofTheDay(a.pictures, a.now())
	if path == "" || path == a.picturePath {
		return
	}
	a.picturePath = path // a broken file is not retried until tomorrow
	w, h := a.disp.Size()
	picture, err := ui.LoadPicture(path, w, h, focusOf(path, a.cfg.Screensaver.Focus))
	if err != nil {
		log.Printf("standby picture %s: %v", filepath.Base(path), err)
		return
	}
	a.picture = picture
}
