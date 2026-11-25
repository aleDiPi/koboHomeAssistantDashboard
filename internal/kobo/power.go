package kobo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	batteryStatus = "/sys/class/power_supply/mc13892_bat/status"
	rtcNow        = "/sys/class/rtc/rtc0/since_epoch"
	rtcAlarm      = "/sys/class/rtc/rtc0/wakealarm"
	powerState    = "/sys/power/state"
	// Netronix switch that prepares the drivers for a suspend.
	powerExtended = "/sys/power/state-extended"
)

// BatteryStatus is the raw charging state, for the log.
func BatteryStatus() string {
	b, _ := os.ReadFile(batteryStatus)
	return strings.TrimSpace(string(b))
}

// Plugged reports whether the device is on external power. Unknown counts as
// plugged, so that a surprise never sends the device to sleep.
func Plugged() bool {
	b, err := os.ReadFile(batteryStatus)
	return err != nil || strings.TrimSpace(string(b)) != "Discharging"
}

func write(path, value string) error {
	return os.WriteFile(path, []byte(value), 0o644)
}

// setAlarm makes the real-time clock wake the device after d.
func setAlarm(d time.Duration) error {
	b, err := os.ReadFile(rtcNow)
	if err != nil {
		return err
	}
	now, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return err
	}
	// A pending alarm must be cleared before another can be set.
	if err := write(rtcAlarm, "0"); err != nil {
		return err
	}
	return write(rtcAlarm, strconv.FormatInt(now+int64(d/time.Second), 10))
}

// wifi runs a function of wifi.sh, next to the program.
func wifi(function string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command("/bin/sh", "-c", ". ./wifi.sh && "+function)
	cmd.Dir = filepath.Dir(exe)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", function, err, out)
	}
	return nil
}

// Suspend puts the device to sleep (to RAM) until the power button is pressed
// or after d. The sequence follows KOReader's Kobo:suspend. The Wi-Fi driver
// must be unloaded first: suspending with it loaded can crash the kernel.
func Suspend(d time.Duration) error {
	if err := wifi("wifi_down"); err != nil {
		// Bring back whatever was half taken down.
		return fmt.Errorf("not suspending: %w (%v)", err, wifi("wifi_up"))
	}
	err := func() error {
		if err := setAlarm(d); err != nil {
			return fmt.Errorf("wake alarm: %w", err)
		}
		if err := write(powerExtended, "1"); err != nil {
			return err
		}
		time.Sleep(2 * time.Second)
		exec.Command("sync").Run()
		// Returns after the device wakes up again.
		err := write(powerState, "mem")
		if e := write(powerExtended, "0"); err == nil {
			err = e
		}
		time.Sleep(100 * time.Millisecond)
		return err
	}()
	if e := wifi("wifi_up"); err == nil {
		err = e
	}
	return err
}
