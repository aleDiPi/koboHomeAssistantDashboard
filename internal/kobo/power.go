package kobo

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	batteryStatus = "/sys/class/power_supply/mc13892_bat/status"
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

// wifi runs a function of wifi.sh, next to the program.
func wifi(function string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)
	// To a file rather than a pipe: the daemons started here keep it open.
	out, err := os.OpenFile(filepath.Join(dir, "wifi.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	fmt.Fprintf(out, "--- %s %s"+string(rune(10)), time.Now().UTC().Format("15:04:05"), function)
	cmd := exec.Command("/bin/sh", "-c", ". ./wifi.sh && "+function)
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, out, out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w (see wifi.log)", function, err)
	}
	return nil
}

// Suspend puts the device to sleep (to RAM) until the power button is pressed
// or after d, and returns when it is awake; call Resume afterwards. The sequence
// follows KOReader's Kobo:suspend. The Wi-Fi driver must be unloaded first:
// suspending with it loaded can crash the kernel.
func Suspend(d time.Duration) error {
	// First, so that without an alarm nothing else is touched.
	if err := setAlarm(d); err != nil {
		return fmt.Errorf("wake alarm: %w", err)
	}
	if err := wifi("wifi_down"); err != nil {
		return fmt.Errorf("not suspending: %w", err)
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
}

// Resume brings Wi-Fi back. It does nothing if it is already up.
func Resume() error { return wifi("wifi_up") }

// Recover takes Wi-Fi down and up again.
func Recover() error {
	down := wifi("wifi_down")
	if err := wifi("wifi_up"); err != nil {
		return errors.Join(err, down)
	}
	return nil
}
