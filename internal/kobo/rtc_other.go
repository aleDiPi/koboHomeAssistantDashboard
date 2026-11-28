//go:build !linux

package kobo

import (
	"errors"
	"time"
)

func setAlarm(time.Duration) error { return errors.New("no real-time clock here") }
