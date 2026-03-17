package detect

import "image"

const LoupeSourceSize = 40

// DetectQuad returns the full image bounding rectangle as four corners ordered:
// top-left, top-right, bottom-right, bottom-left.
func DetectQuad(img image.Image) [4]image.Point {
	b := img.Bounds()
	return [4]image.Point{
		{X: b.Min.X, Y: b.Min.Y},
		{X: b.Max.X, Y: b.Min.Y},
		{X: b.Max.X, Y: b.Max.Y},
		{X: b.Min.X, Y: b.Max.Y},
	}
}
