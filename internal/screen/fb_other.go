//go:build !linux

package screen

import "errors"

// OpenFramebuffer only works on the device; use -sim on other systems.
func OpenFramebuffer(string, int) (Display, error) {
	return nil, errors.New("framebuffer is only available on Linux: use -sim")
}
