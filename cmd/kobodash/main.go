// Command kobodash is a Home Assistant touch dashboard for the Kobo Glo.
package main

import (
	"context"
	"errors"
	"flag"
	"image"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	_ "time/tzdata" // the Kobo has no timezone database

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/app"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/config"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/ha"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/kobo"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/screen"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/sim"
	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/touch"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	exe, _ := os.Executable()
	cfgPath := flag.String("config", filepath.Join(filepath.Dir(exe), "dashboard.ini"), "configuration file")
	simAddr := flag.String("sim", "", "run in the browser on the PC, e.g. 127.0.0.1:8080")
	demo := flag.Bool("demo", false, "use fake data instead of Home Assistant")
	trace := flag.Bool("touchtest", false, "log every raw touch event")
	maxRuntime := flag.Duration("max-runtime", 0, "close after this time (0 = never)")
	wifiPower := flag.String("wifi-power", "", "switch the Wi-Fi chip on or off, then exit")
	flag.Parse()

	if *wifiPower != "" {
		return kobo.SetWifiPower(*wifiPower == "on")
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *maxRuntime > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *maxRuntime)
		defer cancel()
	}

	var src app.Source
	if *demo {
		src = app.NewDemo(cfg)
	} else {
		if cfg.Token == "" {
			return errors.New("no token: set it in [homeassistant] of " + *cfgPath)
		}
		client := ha.NewClient(cfg)
		go client.Run(ctx)
		src = client
	}

	var disp screen.Display
	var taps <-chan image.Point
	var button <-chan struct{}
	if *simAddr != "" {
		disp, taps, err = sim.Start(*simAddr, cfg.Orientation)
		if err != nil {
			return err
		}
		log.Printf("simulator on http://%s", *simAddr)
	} else {
		disp, err = screen.OpenFramebuffer(cfg.Orientation, cfg.ScreenRotation)
		if err != nil {
			return err
		}
		taps = readTouch(cfg, *trace)
		if cfg.Power.Standby {
			button = readButton()
		}
	}
	defer disp.Close()

	dashboard := app.New(cfg, src, disp, loc)
	if picture := cfg.Screensaver.Picture; picture != "" && cfg.Power.Standby {
		if !filepath.IsAbs(picture) {
			picture = filepath.Join(filepath.Dir(*cfgPath), picture)
		}
		dashboard.SetPicture(picture)
	}
	if button != nil {
		dashboard.EnableStandby(button)
	}
	err = dashboard.Run(ctx, taps)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil
	}
	return err
}

// readTouch delivers taps in logical screen coordinates.
func readTouch(cfg config.Config, trace bool) <-chan image.Point {
	raw := make(chan image.Point, 8)
	out := make(chan image.Point, 8)
	t := touch.Transform{
		SwapXY: cfg.TouchSwapXY, MirrorX: cfg.TouchMirrorX, MirrorY: cfg.TouchMirrorY,
		Width: 758, Height: 1024,
		Rotation: screen.Rotation(cfg.Orientation, 758, 1024),
	}
	go func() {
		defer close(raw)
		if err := touch.Read(cfg.TouchDevice, trace, raw); err != nil {
			log.Printf("touch: %v", err)
		}
	}()
	go func() {
		defer close(out)
		for p := range raw {
			q := t.Apply(p)
			if trace {
				log.Printf("touch: tap raw %v -> screen %v", p, q)
			}
			out <- q
		}
	}()
	return out
}

// readButton reports presses of the power button.
func readButton() <-chan struct{} {
	out := make(chan struct{}, 1)
	go func() {
		if err := touch.ReadPowerKey("/dev/input/event0", out); err != nil {
			log.Printf("power button: %v", err)
		}
	}()
	return out
}
