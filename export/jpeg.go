package export

import "image"

// SaveAxisAligned crops img to the rectangle defined by topLeft/bottomRight
// and writes the result as a JPEG (quality 92) to path.
func SaveAxisAligned(img image.Image, topLeft, bottomRight image.Point, path string) error {
	panic("not implemented")
}

// SavePerspective applies a perspective warp to img using the four crop
// corners and writes the result as a JPEG (quality 92) to path.
func SavePerspective(img image.Image, quad [4]image.Point, path string) error {
	panic("not implemented")
}
