package kobo

// The Glo's clock has no sysfs "wakealarm": the alarm is set with the classic
// RTC ioctls on /dev/rtc0, as KOReader does.

import (
	"os"
	"syscall"
	"time"
	"unsafe"
)

const (
	rtcReadTime = 0x80247009 // _IOR('p', 0x09, struct rtc_time)
	rtcSetAlarm = 0x4028700f // _IOW('p', 0x0f, struct rtc_wkalrm)
)

type rtcTime struct {
	Sec, Min, Hour, Mday, Mon, Year, Wday, Yday, Isdst int32
}

type rtcWakeAlarm struct {
	Enabled, Pending uint8
	_                [2]byte
	Time             rtcTime
}

// setAlarm makes the real-time clock wake the device after d.
func setAlarm(d time.Duration) error {
	f, err := os.Open("/dev/rtc0")
	if err != nil {
		return err
	}
	defer f.Close()
	ioctl := func(request uintptr, arg unsafe.Pointer) error {
		if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), request, uintptr(arg)); errno != 0 {
			return errno
		}
		return nil
	}
	var now rtcTime
	if err := ioctl(rtcReadTime, unsafe.Pointer(&now)); err != nil {
		return err
	}
	// Whatever zone the clock is kept in, the arithmetic is the same.
	t := time.Date(int(now.Year)+1900, time.Month(now.Mon+1), int(now.Mday),
		int(now.Hour), int(now.Min), int(now.Sec), 0, time.UTC).Add(d)
	alarm := rtcWakeAlarm{Enabled: 1, Time: rtcTime{
		Sec: int32(t.Second()), Min: int32(t.Minute()), Hour: int32(t.Hour()),
		Mday: int32(t.Day()), Mon: int32(t.Month() - 1), Year: int32(t.Year() - 1900),
	}}
	return ioctl(rtcSetAlarm, unsafe.Pointer(&alarm))
}
