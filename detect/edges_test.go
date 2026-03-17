package detect

import (
	"image"
	"testing"
)

func TestDetectQuad_FullBounds(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 200, 150))
	quad := DetectQuad(img)
	want := [4]image.Point{
		{X: 0, Y: 0},
		{X: 200, Y: 0},
		{X: 200, Y: 150},
		{X: 0, Y: 150},
	}
	if quad != want {
		t.Errorf("got %v, want %v", quad, want)
	}
}
