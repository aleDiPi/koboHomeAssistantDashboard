package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPicturesTakeTurnsDayByDay(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"b.png", "a@0.1.jpg", "notes.txt", "._a.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	list, err := listPictures(dir)
	if err != nil || len(list) != 2 || filepath.Base(list[0]) != "a@0.1.jpg" {
		t.Fatalf("%v %v", list, err)
	}
	day := time.Date(2026, 11, 25, 0, 30, 0, 0, time.FixedZone("cet", 3600))
	today := ofTheDay(list, day)
	if ofTheDay(list, day.Add(23*time.Hour)) != today {
		t.Error("changed during the day")
	}
	if ofTheDay(list, day.AddDate(0, 0, 1)) == today || ofTheDay(list, day.AddDate(0, 0, 2)) != today {
		t.Error("should alternate")
	}
	if f := focusOf(list[0], 0.5); f != 0.1 {
		t.Errorf("focus %v", f)
	}
	if f := focusOf(list[1], 0.5); f != 0.5 {
		t.Errorf("default focus %v", f)
	}
}
