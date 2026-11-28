package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/ui"
)

type fakeUpdater struct {
	installs, confirms int
	err                error
}

func (f *fakeUpdater) Check(context.Context) (int64, bool, error) { return 2, true, nil }
func (f *fakeUpdater) Install(context.Context) error              { f.installs++; return f.err }
func (f *fakeUpdater) Confirm() error                             { f.confirms++; return nil }

func TestUpdateIsOfferedAndNeedsTwoTaps(t *testing.T) {
	for _, failure := range []error{nil, errors.New("bad signature")} {
		a, _, _ := testApp(ui.Home)
		u := &fakeUpdater{err: failure}
		a.SetUpdater(u, time.Hour)
		a.checkUpdate(context.Background(), time.Now())
		a.updateReady = <-a.found
		a.checkUpdate(context.Background(), time.Now())
		if u.confirms != 1 {
			t.Fatalf("confirmed %d times", u.confirms)
		}
		a.render(false)
		button := find(t, a.frame, ui.Update)

		if exit, _ := a.tap(centre(button.Rect)); exit || u.installs != 0 {
			t.Fatal("installed on the first tap")
		}
		a.render(false)
		exit, _ := a.tap(centre(button.Rect))
		if u.installs != 1 || exit != (failure == nil) || a.restart != (failure == nil) {
			t.Errorf("failure %v: installs %d, exit %v, restart %v", failure, u.installs, exit, a.restart)
		}
	}
}
