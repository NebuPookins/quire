package ui

import (
	"image"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// CropOverlay is a custom Fyne widget that displays the scanned image with
// a draggable crop box and an optional loupe overlay.
// The full rendering and interaction implementation is in Step 7.
type CropOverlay struct {
	widget.BaseWidget

	img      image.Image
	cropPts  [4]image.Point
	freeQuad bool
}

// NewCropOverlay constructs a CropOverlay.
func NewCropOverlay() *CropOverlay {
	w := &CropOverlay{}
	w.ExtendBaseWidget(w)
	return w
}

// SetImage updates the displayed image and triggers a refresh.
func (c *CropOverlay) SetImage(img image.Image) {
	c.img = img
	c.Refresh()
}

// SetCrop updates the crop quad (in image coordinates) and triggers a refresh.
func (c *CropOverlay) SetCrop(pts [4]image.Point, freeQuad bool) {
	c.cropPts = pts
	c.freeQuad = freeQuad
	c.Refresh()
}

// SetFreeQuad switches between axis-aligned and free-quad modes.
func (c *CropOverlay) SetFreeQuad(fq bool) {
	c.freeQuad = fq
	c.Refresh()
}

// CurrentCrop returns the current crop quad in image coordinates.
func (c *CropOverlay) CurrentCrop() [4]image.Point {
	return c.cropPts
}

// CreateRenderer implements fyne.Widget.
// This is a minimal placeholder; the full implementation is in Step 7.
func (c *CropOverlay) CreateRenderer() fyne.WidgetRenderer {
	label := widget.NewLabel("Press Scan to begin.")
	label.Alignment = fyne.TextAlignCenter
	return widget.NewSimpleRenderer(label)
}
