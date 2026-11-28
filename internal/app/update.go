package app

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/screen"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/ui"
)

// ErrRestart ends Run after a new version has been installed.
var ErrRestart = errors.New("restart into the new version")

// Updater is implemented by update.Updater.
type Updater interface {
	Check(context.Context) (version int64, newer bool, err error)
	Install(context.Context) error
	Confirm() error
}

const installLimit = 5 * time.Minute

// SetUpdater offers new versions on screen, looking for one every interval.
func (a *App) SetUpdater(u Updater, every time.Duration) {
	a.updater, a.checkEvery = u, every
	a.found = make(chan bool, 1)
}

// checkUpdate is called regularly while awake. The download runs apart, so
// that touches are served meanwhile.
func (a *App) checkUpdate(ctx context.Context, now time.Time) {
	if a.updater == nil || a.dark {
		return
	}
	if _, connected, _ := a.src.Snapshot(); !connected {
		return
	}
	if !a.confirmedVersion {
		// Reaching Home Assistant is the proof that this version works.
		a.confirmedVersion = true
		if err := a.updater.Confirm(); err != nil {
			log.Printf("update: %v", err)
		}
	}
	if now.Before(a.nextCheck) {
		return
	}
	a.nextCheck = now.Add(a.checkEvery)
	go func() {
		version, newer, err := a.updater.Check(ctx)
		if err != nil {
			log.Printf("update check: %v", err)
			return
		}
		if newer {
			log.Printf("version %d is available", version)
		}
		select {
		case a.found <- newer:
		default:
		}
	}()
}

// install replaces the program, showing what is going on.
func (a *App) install() bool {
	w, h := a.disp.Size()
	a.frame = ui.RenderUpdating(w, h)
	a.show(a.frame.Image.Rect, screen.Flash)
	ctx, cancel := context.WithTimeout(context.Background(), installLimit)
	defer cancel()
	if err := a.updater.Install(ctx); err != nil {
		log.Printf("update failed: %v", err)
		a.updateFailed = true
		return false
	}
	log.Print("update installed, restarting")
	return true
}
