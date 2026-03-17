package detect

import "image"

const (
	CannyLow        = 75
	CannyHigh       = 200
	LoupeSourceSize = 40
)

// DetectQuad finds the largest 4-sided contour in img and returns its
// corners ordered: top-left, top-right, bottom-right, bottom-left.
// Falls back to the full image bounding rect if no quad is found.
func DetectQuad(img image.Image) [4]image.Point {
	panic("not implemented")
}
