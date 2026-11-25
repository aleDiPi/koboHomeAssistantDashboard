// Package kobo drives the hardware specific to the device: frontlight, Wi-Fi
// power and battery.
package kobo

import (
	"os"
	"strconv"
	"strings"
)

const batteryCapacity = "/sys/class/power_supply/mc13892_bat/capacity"

// Battery returns the charge in percent, or -1 if unknown.
func Battery() int {
	b, err := os.ReadFile(batteryCapacity)
	if err != nil {
		return -1
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return -1
	}
	return v
}
