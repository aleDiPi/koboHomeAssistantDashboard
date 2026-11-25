//go:build !linux

package touch

import (
	"errors"
	"image"
)

// Read only works on the device.
func Read(string, bool, chan<- image.Point) error {
	return errors.New("touch input is only available on Linux")
}

// ReadPowerKey only works on the device.
func ReadPowerKey(string, chan<- struct{}) error {
	return errors.New("the power button is only available on Linux")
}
