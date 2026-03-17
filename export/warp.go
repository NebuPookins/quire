package export

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

// perspectiveWarp remaps src so that the four corners of srcQuad (ordered
// TL, TR, BR, BL) map to the corners of an outW×outH output image.
// Uses an inverse homography with bilinear interpolation.
func perspectiveWarp(src image.Image, srcQuad [4]image.Point, outW, outH int) (*image.RGBA, error) {
	// Inverse homography: for each output pixel (u,v) find its source (x,y).
	from := [4][2]float64{
		{0, 0},
		{float64(outW), 0},
		{float64(outW), float64(outH)},
		{0, float64(outH)},
	}
	to := [4][2]float64{
		{float64(srcQuad[0].X), float64(srcQuad[0].Y)},
		{float64(srcQuad[1].X), float64(srcQuad[1].Y)},
		{float64(srcQuad[2].X), float64(srcQuad[2].Y)},
		{float64(srcQuad[3].X), float64(srcQuad[3].Y)},
	}
	H, err := solveHomography(from, to)
	if err != nil {
		return nil, fmt.Errorf("compute homography: %w", err)
	}

	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, outW, outH))
	for v := 0; v < outH; v++ {
		for u := 0; u < outW; u++ {
			sx, sy := applyHomography(H, float64(u), float64(v))
			out.SetRGBA(u, v, sampleBilinear(src, sx, sy, b))
		}
	}
	return out, nil
}

// solveHomography computes the 3×3 homography H (stored row-major, H[8]=1)
// that maps from[i] → to[i] for all four point pairs.
//
// Each correspondence contributes two rows to an 8×8 linear system derived
// from the perspective projection equations with H[8] fixed to 1:
//
//	[ u  v  1  0  0  0  −u·x  −v·x ] · h = x
//	[ 0  0  0  u  v  1  −u·y  −v·y ] · h = y
func solveHomography(from, to [4][2]float64) ([9]float64, error) {
	var A [8][9]float64 // augmented matrix [A|b], b in column 8
	for i := range from {
		u, v := from[i][0], from[i][1]
		x, y := to[i][0], to[i][1]
		A[2*i] = [9]float64{u, v, 1, 0, 0, 0, -u * x, -v * x, x}
		A[2*i+1] = [9]float64{0, 0, 0, u, v, 1, -u * y, -v * y, y}
	}

	// Gaussian elimination with partial pivoting.
	for col := 0; col < 8; col++ {
		pivot := col
		for row := col + 1; row < 8; row++ {
			if math.Abs(A[row][col]) > math.Abs(A[pivot][col]) {
				pivot = row
			}
		}
		A[col], A[pivot] = A[pivot], A[col]
		if math.Abs(A[col][col]) < 1e-12 {
			return [9]float64{}, fmt.Errorf("degenerate point configuration")
		}
		for row := col + 1; row < 8; row++ {
			f := A[row][col] / A[col][col]
			for k := col; k <= 8; k++ {
				A[row][k] -= f * A[col][k]
			}
		}
	}

	// Back substitution.
	var h [8]float64
	for i := 7; i >= 0; i-- {
		h[i] = A[i][8]
		for j := i + 1; j < 8; j++ {
			h[i] -= A[i][j] * h[j]
		}
		h[i] /= A[i][i]
	}
	return [9]float64{h[0], h[1], h[2], h[3], h[4], h[5], h[6], h[7], 1}, nil
}

// applyHomography projects point (u, v) through H and returns (x, y).
func applyHomography(H [9]float64, u, v float64) (float64, float64) {
	w := H[6]*u + H[7]*v + H[8]
	return (H[0]*u + H[1]*v + H[2]) / w,
		(H[3]*u + H[4]*v + H[5]) / w
}

// sampleBilinear returns the bilinearly-interpolated colour at (x, y),
// clamping out-of-bounds coordinates to the image boundary.
func sampleBilinear(src image.Image, x, y float64, b image.Rectangle) color.RGBA {
	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	fx := x - float64(x0)
	fy := y - float64(y0)

	c00 := sampleAt(src, x0, y0, b)
	c10 := sampleAt(src, x0+1, y0, b)
	c01 := sampleAt(src, x0, y0+1, b)
	c11 := sampleAt(src, x0+1, y0+1, b)

	lerp := func(a, b uint8, t float64) uint8 {
		return uint8(float64(a)*(1-t)+float64(b)*t + 0.5)
	}
	blend := func(c00, c10, c01, c11 uint8) uint8 {
		return lerp(lerp(c00, c10, fx), lerp(c01, c11, fx), fy)
	}
	return color.RGBA{
		R: blend(c00.R, c10.R, c01.R, c11.R),
		G: blend(c00.G, c10.G, c01.G, c11.G),
		B: blend(c00.B, c10.B, c01.B, c11.B),
		A: blend(c00.A, c10.A, c01.A, c11.A),
	}
}

// sampleAt returns the colour of src at (x, y), clamped to b.
// Type-switches on the image types produced by the scanner package
// (*image.RGBA for colour, *image.Gray for grayscale/lineart) to avoid
// the overhead of a virtual colour conversion call per pixel.
func sampleAt(src image.Image, x, y int, b image.Rectangle) color.RGBA {
	if x < b.Min.X {
		x = b.Min.X
	} else if x >= b.Max.X {
		x = b.Max.X - 1
	}
	if y < b.Min.Y {
		y = b.Min.Y
	} else if y >= b.Max.Y {
		y = b.Max.Y - 1
	}
	switch img := src.(type) {
	case *image.RGBA:
		i := img.PixOffset(x, y)
		return color.RGBA{img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]}
	case *image.Gray:
		g := img.Pix[img.PixOffset(x, y)]
		return color.RGBA{g, g, g, 0xff}
	default:
		r, g, bv, a := src.At(x, y).RGBA()
		return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bv >> 8), uint8(a >> 8)}
	}
}
