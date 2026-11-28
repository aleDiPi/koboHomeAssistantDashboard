//go:build !linux

package kobo

// SetFrontlight does nothing away from the device.
func SetFrontlight(int) error { return nil }

// SetWifiPower does nothing away from the device.
func SetWifiPower(bool) error { return nil }
