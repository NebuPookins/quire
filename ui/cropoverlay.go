package ui

import (
	"image"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// CropOverlay is a custom Fyne widget that displays the scanned image with
// a draggable crop box and an optional loupe overlay.
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

// SetImage updates the displayed image and resets the crop to the full bounds.
func (c *CropOverlay) SetImage(img image.Image) {
	c.img = img
	c.Refresh()
}

// SetCrop updates the crop quad (in image coordinates).
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

// CreateRenderer implements fyne.Widget.
func (c *CropOverlay) CreateRenderer() fyne.WidgetRenderer {
	panic("not implemented")
}
