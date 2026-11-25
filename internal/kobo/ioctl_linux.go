package kobo

import (
	"os"
	"syscall"
)

// Commands of the Netronix driver behind /dev/ntx_io.
const (
	cmWifiPower  = 208
	cmFrontLight = 241
)

func ntx(command, value int) error {
	f, err := os.OpenFile("/dev/ntx_io", os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	// The value is the argument itself, not a pointer to it.
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(command), uintptr(value)); errno != 0 {
		return errno
	}
	return nil
}

// SetFrontlight sets the brightness, 0 (off) to 100.
func SetFrontlight(percent int) error { return ntx(cmFrontLight, min(max(percent, 0), 100)) }

// SetWifiPower switches the Wi-Fi chip on or off.
func SetWifiPower(on bool) error {
	v := 0
	if on {
		v = 1
	}
	return ntx(cmWifiPower, v)
}
