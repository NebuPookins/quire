package ui

import (
	"image"
	"image/color"
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestInvertImage_Gray(t *testing.T) {
	src := image.NewGray(image.Rect(0, 0, 3, 2))
	src.SetGray(0, 0, color.Gray{Y: 0})
	src.SetGray(1, 0, color.Gray{Y: 255})
	src.SetGray(2, 1, color.Gray{Y: 10})

	got := invertImage(src)
	if got.Bounds() != src.Bounds() {
		t.Fatalf("bounds = %v, want %v", got.Bounds(), src.Bounds())
	}
	cases := []struct {
		x, y int
		want uint32
	}{
		{0, 0, 255}, // black → white
		{1, 0, 0},   // white → black
		{2, 1, 245},
	}
	for _, tc := range cases {
		r, _, _, _ := got.At(tc.x, tc.y).RGBA()
		if r>>8 != tc.want {
			t.Errorf("pixel (%d,%d) = %d, want %d", tc.x, tc.y, r>>8, tc.want)
		}
	}
	// Source must be left unmodified.
	if src.GrayAt(0, 0).Y != 0 {
		t.Error("invertImage modified its input")
	}
}

func TestInvertImage_RGBA_PreservesAlpha(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1, 1))
	src.SetRGBA(0, 0, color.RGBA{R: 200, G: 50, B: 30, A: 255})

	got := invertImage(src)
	r, g, b, a := got.At(0, 0).RGBA()
	if r>>8 != 55 || g>>8 != 205 || b>>8 != 225 {
		t.Errorf("got r=%d g=%d b=%d, want r=55 g=205 b=225", r>>8, g>>8, b>>8)
	}
	if a>>8 != 255 {
		t.Errorf("alpha = %d, want 255 (alpha must not be inverted)", a>>8)
	}
}

func TestInvertImage_Involution(t *testing.T) {
	src := image.NewGray(image.Rect(0, 0, 4, 4))
	for i := range src.Pix {
		src.Pix[i] = uint8(i * 16)
	}
	twice := invertImage(invertImage(src))
	for y := range 4 {
		for x := range 4 {
			wr, _, _, _ := src.At(x, y).RGBA()
			gr, _, _, _ := twice.At(x, y).RGBA()
			if wr != gr {
				t.Fatalf("pixel (%d,%d): double inversion = %d, want original %d", x, y, gr>>8, wr>>8)
			}
		}
	}
}

// TestInvertToggle verifies that toggling the Invert colors checkbox inverts
// the already-scanned image (and toggling it back restores the original),
// without disturbing the crop.
func TestInvertToggle(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a := test.NewApp()
	defer a.Quit()
	mw := NewMainWindow(a)

	img := image.NewGray(image.Rect(0, 0, 100, 80))
	for i := range img.Pix {
		img.Pix[i] = 10
	}
	mw.applyScanResult(img)
	userCrop := [4]image.Point{{10, 10}, {90, 10}, {90, 70}, {10, 70}}
	mw.cropOverlay.SetCrop(userCrop, false)

	mw.invertChk.SetChecked(true)
	r, _, _, _ := mw.scannedImage.At(0, 0).RGBA()
	if r>>8 != 245 {
		t.Errorf("after invert: pixel = %d, want 245", r>>8)
	}
	if got := mw.cropOverlay.CurrentCrop(); got != userCrop {
		t.Errorf("after invert: crop = %v, want %v", got, userCrop)
	}

	mw.invertChk.SetChecked(false)
	r, _, _, _ = mw.scannedImage.At(0, 0).RGBA()
	if r>>8 != 10 {
		t.Errorf("after un-invert: pixel = %d, want original 10", r>>8)
	}
}

// TestInvertSettingRestoredFromConfig verifies that a persisted invert_colors
// setting is reflected in the checkbox on startup.
func TestInvertSettingRestoredFromConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	a := test.NewApp()
	defer a.Quit()
	mw := NewMainWindow(a)
	mw.invertChk.SetChecked(true)
	a.Quit()

	b := test.NewApp()
	defer b.Quit()
	mw2 := NewMainWindow(b)
	if !mw2.invertChk.Checked {
		t.Error("invert checkbox not restored from persisted config")
	}
}
