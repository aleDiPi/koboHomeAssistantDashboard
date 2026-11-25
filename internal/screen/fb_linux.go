package screen

// Framebuffer + EPDC driver for Kobo "Mark 4" devices (i.MX50 SoC, e.g. the
// Glo). Structures and ioctl numbers follow FBInk's eink/mxcfb-kobo.h.

import (
	"fmt"
	"image"
	"log"
	"os"
	"syscall"
	"unsafe"
)

const (
	fbioGetVScreenInfo = 0x4600
	fbioGetFScreenInfo = 0x4602

	// _IOW('F', 0x2E, struct mxcfb_update_data_v1_ntx), 68 bytes
	mxcfbSendUpdate = 0x4044462E
	// _IOW('F', 0x2F, uint32_t)
	mxcfbWaitForUpdate = 0x4004462F

	wfmDU   = 1
	wfmGC16 = 2

	updatePartial  = 0
	updateFull     = 1
	tempUseAmbient = 0x1000
)

type bitfield struct{ Offset, Length, MSBRight uint32 }

type varScreenInfo struct {
	XRes, YRes, XResVirtual, YResVirtual, XOffset, YOffset uint32
	BitsPerPixel, Grayscale                                uint32
	Red, Green, Blue, Transp                               bitfield
	NonStd, Activate, Height, Width, AccelFlags            uint32
	PixClock, LeftMargin, RightMargin, UpperMargin         uint32
	LowerMargin, HSyncLen, VSyncLen, Sync, VMode           uint32
	Rotate, Colorspace                                     uint32
	Reserved                                               [4]uint32
}

type fixScreenInfo struct {
	ID                            [16]byte
	SmemStart                     uintptr
	SmemLen, Type, TypeAux        uint32
	Visual                        uint32
	XPanStep, YPanStep, YWrapStep uint16
	LineLength                    uint32
	MMIOStart                     uintptr
	MMIOLen, Accel                uint32
	Capabilities                  uint16
	Reserved                      [2]uint16
}

type rect struct{ Top, Left, Width, Height uint32 }

type altBufferData struct {
	VirtAddr, PhysAddr, Width, Height uint32
	AltUpdateRegion                   rect
}

type updateData struct {
	UpdateRegion                           rect
	WaveformMode, UpdateMode, UpdateMarker uint32
	Temp                                   int32
	Flags                                  uint32
	AltBufferData                          altBufferData
}

type framebuffer struct {
	f           *os.File
	mem         []byte
	vi          varScreenInfo
	stride      int
	rotation    int
	logicalW    int
	logicalH    int
	marker      uint32
	bytesPerPix int
}

func ioctl(fd uintptr, req uintptr, arg unsafe.Pointer) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg)); errno != 0 {
		return errno
	}
	return nil
}

// OpenFramebuffer opens /dev/fb0. rotation < 0 derives it from orientation.
func OpenFramebuffer(orientation string, rotation int) (Display, error) {
	f, err := os.OpenFile("/dev/fb0", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	d := &framebuffer{f: f, marker: uint32(os.Getpid())}
	var fix fixScreenInfo
	if err := ioctl(f.Fd(), fbioGetVScreenInfo, unsafe.Pointer(&d.vi)); err != nil {
		f.Close()
		return nil, fmt.Errorf("FBIOGET_VSCREENINFO: %w", err)
	}
	if err := ioctl(f.Fd(), fbioGetFScreenInfo, unsafe.Pointer(&fix)); err != nil {
		f.Close()
		return nil, fmt.Errorf("FBIOGET_FSCREENINFO: %w", err)
	}
	switch d.vi.BitsPerPixel {
	case 8, 16, 32:
		d.bytesPerPix = int(d.vi.BitsPerPixel / 8)
	default:
		f.Close()
		return nil, fmt.Errorf("unsupported depth: %d bpp", d.vi.BitsPerPixel)
	}
	d.stride = int(fix.LineLength)
	d.mem, err = syscall.Mmap(int(f.Fd()), 0, int(fix.SmemLen), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("mmap: %w", err)
	}

	fbW, fbH := int(d.vi.XRes), int(d.vi.YRes)
	if rotation < 0 {
		rotation = Rotation(orientation, fbW, fbH)
	}
	d.rotation = rotation
	d.logicalW, d.logicalH = fbW, fbH
	if rotation == 90 || rotation == 270 {
		d.logicalW, d.logicalH = fbH, fbW
	}
	log.Printf("framebuffer %dx%d, %d bpp, stride %d, fb rotate %d, software rotation %d",
		fbW, fbH, d.vi.BitsPerPixel, d.stride, d.vi.Rotate, rotation)
	return d, nil
}

func (d *framebuffer) Size() (int, int) { return d.logicalW, d.logicalH }

func (d *framebuffer) Show(img *image.Gray, r image.Rectangle, mode Mode) error {
	r = r.Intersect(image.Rect(0, 0, d.logicalW, d.logicalH))
	// The EPDC can lock up on degenerate regions.
	if r.Dx() <= 1 || r.Dy() <= 1 {
		return nil
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			p := RotatePoint(image.Pt(x, y), d.logicalW, d.logicalH, d.rotation)
			d.set(p.X+int(d.vi.XOffset), p.Y+int(d.vi.YOffset), img.GrayAt(x, y).Y)
		}
	}

	pr := Rotate(r, d.logicalW, d.logicalH, d.rotation)
	d.marker++
	u := updateData{
		UpdateRegion: rect{
			Top: uint32(pr.Min.Y), Left: uint32(pr.Min.X),
			Width: uint32(pr.Dx()), Height: uint32(pr.Dy()),
		},
		WaveformMode: wfmGC16,
		UpdateMode:   updatePartial,
		UpdateMarker: d.marker,
		Temp:         tempUseAmbient,
	}
	switch mode {
	case Fast:
		u.WaveformMode = wfmDU
	case Flash:
		u.UpdateMode = updateFull
	}
	if err := ioctl(d.f.Fd(), mxcfbSendUpdate, unsafe.Pointer(&u)); err != nil {
		return fmt.Errorf("MXCFB_SEND_UPDATE: %w", err)
	}
	if mode == Flash {
		marker := d.marker
		if err := ioctl(d.f.Fd(), mxcfbWaitForUpdate, unsafe.Pointer(&marker)); err != nil {
			log.Printf("MXCFB_WAIT_FOR_UPDATE_COMPLETE: %v", err)
		}
	}
	return nil
}

func (d *framebuffer) set(x, y int, g uint8) {
	off := y*d.stride + x*d.bytesPerPix
	if off < 0 || off+d.bytesPerPix > len(d.mem) {
		return
	}
	switch d.bytesPerPix {
	case 1:
		d.mem[off] = g
	case 2: // RGB565
		v := uint16(g>>3)<<11 | uint16(g>>2)<<5 | uint16(g>>3)
		d.mem[off] = byte(v)
		d.mem[off+1] = byte(v >> 8)
	case 4:
		d.mem[off], d.mem[off+1], d.mem[off+2], d.mem[off+3] = g, g, g, 0xFF
	}
}

func (d *framebuffer) Close() error {
	syscall.Munmap(d.mem)
	return d.f.Close()
}
