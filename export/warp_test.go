package export

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// TestSolveHomography_Identity verifies that a mapping where src == dst
// produces a matrix that reproduces every input point exactly.
func TestSolveHomography_Identity(t *testing.T) {
	pts := [4][2]float64{{0, 0}, {100, 0}, {100, 80}, {0, 80}}
	H, err := solveHomography(pts, pts)
	if err != nil {
		t.Fatalf("solveHomography: %v", err)
	}
	for _, p := range pts {
		x, y := applyHomography(H, p[0], p[1])
		if math.Abs(x-p[0]) > 1e-6 || math.Abs(y-p[1]) > 1e-6 {
			t.Errorf("(%g,%g) → (%g,%g), want identity", p[0], p[1], x, y)
		}
	}
}

// TestSolveHomography_Translate verifies a pure translation.
func TestSolveHomography_Translate(t *testing.T) {
	dx, dy := 30.0, 20.0
	from := [4][2]float64{{0, 0}, {100, 0}, {100, 80}, {0, 80}}
	to := [4][2]float64{
		{from[0][0] + dx, from[0][1] + dy},
		{from[1][0] + dx, from[1][1] + dy},
		{from[2][0] + dx, from[2][1] + dy},
		{from[3][0] + dx, from[3][1] + dy},
	}
	H, err := solveHomography(from, to)
	if err != nil {
		t.Fatalf("solveHomography: %v", err)
	}
	for i, p := range from {
		x, y := applyHomography(H, p[0], p[1])
		if math.Abs(x-to[i][0]) > 1e-6 || math.Abs(y-to[i][1]) > 1e-6 {
			t.Errorf("point %d: got (%g,%g), want (%g,%g)", i, x, y, to[i][0], to[i][1])
		}
	}
}

// TestPerspectiveWarp_SolidColour verifies that warping a solid-colour image
// with an axis-aligned quad produces a solid-colour output.
func TestPerspectiveWarp_SolidColour(t *testing.T) {
	red := color.RGBA{R: 200, G: 50, B: 30, A: 255}
	src := makeSolidRGBA(200, 200, red)
	quad := [4]image.Point{{20, 10}, {180, 10}, {180, 190}, {20, 190}}

	out, err := perspectiveWarp(src, quad, 160, 180)
	if err != nil {
		t.Fatalf("perspectiveWarp: %v", err)
	}

	// Every output pixel should be red (source is uniformly red).
	for y := 0; y < 180; y++ {
		for x := 0; x < 160; x++ {
			c := out.RGBAAt(x, y)
			if c.R != red.R || c.G != red.G || c.B != red.B {
				t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, c, red)
			}
		}
	}
}

// TestPerspectiveWarp_CornerMapping verifies that corner pixels in the output
// correspond to the expected source corners (within bilinear rounding).
func TestPerspectiveWarp_CornerMapping(t *testing.T) {
	// Source: quadrants of distinct flat colours.
	src := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			switch {
			case x < 50 && y < 50:
				src.SetRGBA(x, y, color.RGBA{255, 0, 0, 255}) // TL: red
			case x >= 50 && y < 50:
				src.SetRGBA(x, y, color.RGBA{0, 255, 0, 255}) // TR: green
			case x < 50 && y >= 50:
				src.SetRGBA(x, y, color.RGBA{0, 0, 255, 255}) // BL: blue
			default:
				src.SetRGBA(x, y, color.RGBA{255, 255, 0, 255}) // BR: yellow
			}
		}
	}

	// Axis-aligned quad covering only the top-left red quadrant.
	quad := [4]image.Point{{0, 0}, {50, 0}, {50, 50}, {0, 50}}
	out, err := perspectiveWarp(src, quad, 50, 50)
	if err != nil {
		t.Fatalf("perspectiveWarp: %v", err)
	}

	// Centre of output should be fully red.
	c := out.RGBAAt(25, 25)
	if c.R != 255 || c.G != 0 || c.B != 0 {
		t.Errorf("centre pixel = %v, want red", c)
	}
}
