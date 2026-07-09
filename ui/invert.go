package ui

import (
	"image"
	"image/color"
)

// invertImage returns a photographic negative of src, leaving src unmodified.
// The result has the same bounds as src. Grayscale input stays grayscale;
// colour input keeps its alpha channel unchanged.
func invertImage(src image.Image) image.Image {
	switch s := src.(type) {
	case *image.Gray:
		dst := image.NewGray(s.Rect)
		for y := range s.Rect.Dy() {
			srow := s.Pix[y*s.Stride : y*s.Stride+s.Rect.Dx()]
			drow := dst.Pix[y*dst.Stride:]
			for i, p := range srow {
				drow[i] = 0xff - p
			}
		}
		return dst
	case *image.RGBA:
		dst := image.NewRGBA(s.Rect)
		for y := range s.Rect.Dy() {
			srow := s.Pix[y*s.Stride : y*s.Stride+s.Rect.Dx()*4]
			drow := dst.Pix[y*dst.Stride:]
			for i, p := range srow {
				if i%4 == 3 {
					drow[i] = p // alpha
				} else {
					drow[i] = 0xff - p
				}
			}
		}
		return dst
	default:
		b := src.Bounds()
		dst := image.NewRGBA(b)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, bl, a := src.At(x, y).RGBA()
				dst.SetRGBA(x, y, color.RGBA{
					R: uint8(0xff - r>>8),
					G: uint8(0xff - g>>8),
					B: uint8(0xff - bl>>8),
					A: uint8(a >> 8),
				})
			}
		}
		return dst
	}
}
