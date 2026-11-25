package kobo

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

const cpufreq = "/sys/devices/system/cpu/cpu0/cpufreq/"

// governor picks how the processor should scale: the most frugal governor that
// still speeds up when there is something to draw, or the original one.
func governor(available []string, saving bool, original string) string {
	if saving {
		for _, g := range []string{"conservative", "ondemand"} {
			if slices.Contains(available, g) {
				return g
			}
		}
	}
	return original
}

var originalGovernor string

// PowerSave lets the processor and the Wi-Fi radio rest between uses, or puts
// them back. The result says what was done, for the log; what the device does
// not support is skipped.
func PowerSave(saving bool) string {
	var done []string
	read := func(name string) string {
		b, _ := os.ReadFile(cpufreq + name)
		return strings.TrimSpace(string(b))
	}
	if current := read("scaling_governor"); current != "" {
		if originalGovernor == "" {
			originalGovernor = current
		}
		g := governor(strings.Fields(read("scaling_available_governors")), saving, originalGovernor)
		if err := write(cpufreq+"scaling_governor", g); err != nil {
			done = append(done, "cpu: "+err.Error())
		} else {
			done = append(done, "cpu "+g)
		}
	}
	mode := "off"
	if saving {
		mode = "on"
	}
	if err := wifi("wifi_powersave " + mode); err != nil {
		done = append(done, err.Error())
	} else {
		done = append(done, fmt.Sprintf("wifi power saving %s", mode))
	}
	return strings.Join(done, ", ")
}
