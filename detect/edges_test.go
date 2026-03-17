package detect

import (
	"image"
	"image/color"
	"testing"
)

// makeRectImage returns a w×h grayscale image with a filled white rectangle
// at (rx0,ry0)-(rx1,ry1) on a black background.
func makeRectImage(w, h, rx0, ry0, rx1, ry1 int) image.Image {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := ry0; y < ry1; y++ {
		for x := rx0; x < rx1; x++ {
			img.SetGray(x, y, color.Gray{Y: 255})
		}
	}
	return img
}

func withinTolerance(a, b, tol int) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func TestDetectQuad_Rectangle(t *testing.T) {
	// White rectangle on black background — Canny should find the edges cleanly.
	img := makeRectImage(300, 300, 50, 50, 250, 250)
	quad := DetectQuad(img)

	// Expected order: top-left, top-right, bottom-right, bottom-left.
	want := [4]image.Point{
		{X: 50, Y: 50},
		{X: 250, Y: 50},
		{X: 250, Y: 250},
		{X: 50, Y: 250},
	}
	const tol = 5
	for i, got := range quad {
		if !withinTolerance(got.X, want[i].X, tol) || !withinTolerance(got.Y, want[i].Y, tol) {
			t.Errorf("corner %d: got %v, want ~%v (tolerance ±%d)", i, got, want[i], tol)
		}
	}
}

func TestDetectQuad_Fallback(t *testing.T) {
	// Uniform black image — no edges, should fall back to full bounding rect.
	img := image.NewGray(image.Rect(0, 0, 100, 80))
	quad := DetectQuad(img)

	want := [4]image.Point{
		{X: 0, Y: 0},
		{X: 100, Y: 0},
		{X: 100, Y: 80},
		{X: 0, Y: 80},
	}
	if quad != want {
		t.Errorf("fallback: got %v, want %v", quad, want)
	}
}

// --- orderQuad ---

func TestOrderQuad(t *testing.T) {
	// Points supplied in random order; should come out TL, TR, BR, BL.
	pts := []image.Point{
		{X: 200, Y: 200}, // BR
		{X: 10, Y: 10},   // TL
		{X: 10, Y: 200},  // BL
		{X: 200, Y: 10},  // TR
	}
	got := orderQuad(pts)
	want := [4]image.Point{
		{X: 10, Y: 10},
		{X: 200, Y: 10},
		{X: 200, Y: 200},
		{X: 10, Y: 200},
	}
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestOrderQuad_AlreadyOrdered(t *testing.T) {
	pts := []image.Point{{0, 0}, {100, 0}, {100, 80}, {0, 80}}
	got := orderQuad(pts)
	want := [4]image.Point{{0, 0}, {100, 0}, {100, 80}, {0, 80}}
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
