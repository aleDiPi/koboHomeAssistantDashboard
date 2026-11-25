package touch

import (
	"encoding/binary"
	"image"
	"io"
	"log"
	"os"
	"syscall"
	"unsafe"
)

// struct input_event: timeval (two longs), type, code, value.
const (
	timeSize  = int(unsafe.Sizeof(syscall.Timeval{}))
	eventSize = timeSize + 8
	// EVIOCGRAB: take the device exclusively.
	evIOCGrab = 0x40044590
)

// Read sends raw tap positions to taps until the device fails. With trace set
// every event is logged, to find out how an unknown panel behaves.
func Read(path string, trace bool, taps chan<- image.Point) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), evIOCGrab, 1); errno != 0 {
		log.Printf("touch: could not grab %s: %v", path, errno)
	}

	var d Decoder
	buf := make([]byte, eventSize)
	for {
		if _, err := io.ReadFull(f, buf); err != nil {
			return err
		}
		typ := binary.LittleEndian.Uint16(buf[timeSize:])
		code := binary.LittleEndian.Uint16(buf[timeSize+2:])
		value := int32(binary.LittleEndian.Uint32(buf[timeSize+4:]))
		if trace {
			log.Printf("touch: type=%d code=0x%x value=%d", typ, code, value)
		}
		if p, ok := d.Feed(typ, code, value); ok {
			taps <- p
		}
	}
}
