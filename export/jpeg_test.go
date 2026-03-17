package export

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

// makeSolidRGBA returns a w×h RGBA image filled with c.
func makeSolidRGBA(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestSaveAxisAligned_DimensionsAndFile(t *testing.T) {
	src := makeSolidRGBA(200, 200, color.RGBA{R: 255, A: 255})
	path := filepath.Join(t.TempDir(), "out.jpg")

	if err := SaveAxisAligned(src, image.Point{X: 20, Y: 30}, image.Point{X: 120, Y: 180}, path); err != nil {
		t.Fatalf("SaveAxisAligned: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	defer f.Close()

	got, err := jpeg.Decode(f)
	if err != nil {
		t.Fatalf("decode JPEG: %v", err)
	}
	if got.Bounds().Dx() != 100 || got.Bounds().Dy() != 150 {
		t.Errorf("expected 100×150, got %dx%d", got.Bounds().Dx(), got.Bounds().Dy())
	}
}

func TestSaveAxisAligned_AtomicWrite(t *testing.T) {
	// File should not exist mid-write; after success it must exist.
	src := makeSolidRGBA(50, 50, color.RGBA{G: 255, A: 255})
	path := filepath.Join(t.TempDir(), "out.jpg")

	if err := SaveAxisAligned(src, image.Point{}, image.Point{X: 50, Y: 50}, path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file missing after save: %v", err)
	}
}

func TestSavePerspective_DimensionsAndFile(t *testing.T) {
	// A simple axis-aligned quad — warp should produce the expected output size.
	src := makeSolidRGBA(300, 300, color.RGBA{B: 255, A: 255})
	path := filepath.Join(t.TempDir(), "out.jpg")

	quad := [4]image.Point{
		{X: 50, Y: 50},   // TL
		{X: 250, Y: 50},  // TR
		{X: 250, Y: 250}, // BR
		{X: 50, Y: 250},  // BL
	}
	if err := SavePerspective(src, quad, path); err != nil {
		t.Fatalf("SavePerspective: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	defer f.Close()

	got, err := jpeg.Decode(f)
	if err != nil {
		t.Fatalf("decode JPEG: %v", err)
	}
	// width = max(250-50, 250-50) = 200; height = max(250-50, 250-50) = 200
	if got.Bounds().Dx() != 200 || got.Bounds().Dy() != 200 {
		t.Errorf("expected 200×200, got %dx%d", got.Bounds().Dx(), got.Bounds().Dy())
	}
}

func TestSavePerspective_DegenerateQuad(t *testing.T) {
	src := makeSolidRGBA(100, 100, color.RGBA{A: 255})
	path := filepath.Join(t.TempDir(), "out.jpg")

	// Collapsed quad — all same X produces zero width.
	quad := [4]image.Point{{10, 10}, {10, 10}, {10, 90}, {10, 90}}
	if err := SavePerspective(src, quad, path); err == nil {
		t.Fatal("expected error for degenerate quad")
	}
}

func TestCropImage_SubImager(t *testing.T) {
	src := makeSolidRGBA(100, 100, color.RGBA{R: 128, A: 255})
	r := image.Rect(10, 20, 60, 80)
	got := cropImage(src, r)
	if got.Bounds().Dx() != 50 || got.Bounds().Dy() != 60 {
		t.Errorf("expected 50×60, got %v", got.Bounds())
	}
}
