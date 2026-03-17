package scanner

import (
	"errors"
	"image"
)

// ErrScanImageNotFound is returned when scanimage is not on PATH.
var ErrScanImageNotFound = errors.New("scanimage not found on PATH")

// ListDevices returns the SANE device names reported by `scanimage -L`.
func ListDevices() ([]string, error) {
	panic("not implemented")
}

// Scan acquires an image from the given device.
func Scan(device, mode string, resolution int) (image.Image, error) {
	panic("not implemented")
}
