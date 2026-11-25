package kobo

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const powerSupplies = "/sys/class/power_supply"

// volts converts a sysfs voltage, given in microvolts or millivolts.
func volts(raw string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	switch {
	case err != nil || v <= 0:
		return 0
	case v > 100_000:
		return v / 1e6
	case v > 100:
		return v / 1e3
	}
	return v
}

// Voltage is the battery voltage, or 0 if unknown.
func Voltage() float64 {
	b, err := os.ReadFile(powerSupplies + "/mc13892_bat/voltage_now")
	if err != nil {
		return 0
	}
	return volts(string(b))
}

// PowerInfo lists everything the kernel says about the battery and the
// charger, one "supply/attribute=value" per line. It tells which clues about
// the battery's health this device offers.
func PowerInfo() []string {
	var out []string
	files, _ := filepath.Glob(powerSupplies + "/*/*")
	for _, f := range files {
		if filepath.Base(f) == "uevent" {
			continue
		}
		info, err := os.Stat(f)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(powerSupplies, f)
		out = append(out, rel+"="+strings.TrimSpace(string(b)))
	}
	return out
}
