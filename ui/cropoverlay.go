package ui

import (
	"image"
	"image/color"
	"image/draw"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

const (
	handleVisual = 12  // visual handle square side in px
	handleHit    = 24  // hit-test square side in pts
	loupeOutSize = 200 // loupe output size in px
	loupeSrcSize = 40  // loupe source region in image px
	minCropDisp  = 20  // minimum crop dimension in display units
)

// dimColor is the semi-transparent black used to darken the area outside the crop.
var dimColor = image.NewUniform(color.RGBA{A: 165})

// CropOverlay is a custom Fyne widget that displays the scanned image with
// a draggable crop box and an optional loupe overlay.
type CropOverlay struct {
	widget.BaseWidget

	img             image.Image
	cropPts         [4]image.Point
	freeQuad        bool
	activeHandle    int // index of dragged handle, -1 = none
	widgetSize      fyne.Size
	placeholderText string
}

// Verify interface compliance at compile time.
var _ desktop.Mouseable = (*CropOverlay)(nil)
var _ desktop.Hoverable = (*CropOverlay)(nil)
var _ fyne.Draggable = (*CropOverlay)(nil)

// NewCropOverlay constructs a CropOverlay.
func NewCropOverlay() *CropOverlay {
	w := &CropOverlay{activeHandle: -1, placeholderText: "Press Scan to begin."}
	w.ExtendBaseWidget(w)
	return w
}

// SetPlaceholder updates the text shown when no image is loaded.
func (c *CropOverlay) SetPlaceholder(text string) {
	c.placeholderText = text
	c.Refresh()
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

// placeholderLogoW/H are the display dimensions of the logo in the placeholder.
// The logo SVG has a 4:1 aspect ratio (680×170 viewBox).
const (
	placeholderLogoW = 280
	placeholderLogoH = 70
	placeholderGap   = 12 // gap between logo and text
	placeholderTextH = 24
)

// CreateRenderer implements fyne.Widget.
func (c *CropOverlay) CreateRenderer() fyne.WidgetRenderer {
	// bgImage renders the scanned image GPU-accelerated via Fyne's renderer.
	bgImage := canvas.NewImageFromImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	bgImage.FillMode = canvas.ImageFillContain
	bgImage.ScaleMode = canvas.ImageScaleFastest
	bgImage.Hide()

	// raster draws the overlay: dim regions, crop border, handles, loupe.
	raster := canvas.NewRaster(c.generateOverlay)

	logoImage := canvas.NewImageFromResource(AppLogo)
	logoImage.FillMode = canvas.ImageFillContain
	logoImage.ScaleMode = canvas.ImageScaleSmooth

	placeholder := canvas.NewText(c.placeholderText, color.RGBA{R: 180, G: 180, B: 180, A: 255})
	placeholder.Alignment = fyne.TextAlignCenter

	return &cropRenderer{
		overlay:     c,
		bgImage:     bgImage,
		raster:      raster,
		logoImage:   logoImage,
		placeholder: placeholder,
	}
}

// MouseDown implements desktop.Mouseable.
func (c *CropOverlay) MouseDown(ev *desktop.MouseEvent) {
	if ev.Button != desktop.MouseButtonPrimary {
		return
	}
	c.activeHandle = c.hitTestHandle(ev.Position)
}

// MouseUp implements desktop.Mouseable.
func (c *CropOverlay) MouseUp(_ *desktop.MouseEvent) {}

// Dragged implements fyne.Draggable. Fyne continues delivering Dragged events
// even when the pointer moves outside the widget boundary, so the user can
// drag a handle to the image edge without the drag cutting out.
func (c *CropOverlay) Dragged(ev *fyne.DragEvent) {
	if c.activeHandle < 0 || c.img == nil {
		return
	}
	c.applyDrag(ev.Position)
	c.Refresh()
}

// DragEnd implements fyne.Draggable.
func (c *CropOverlay) DragEnd() {
	c.activeHandle = -1
	c.Refresh()
}

// MouseIn implements desktop.Hoverable.
func (c *CropOverlay) MouseIn(_ *desktop.MouseEvent) {}

// MouseOut implements desktop.Hoverable.
func (c *CropOverlay) MouseOut() {}

// MouseMoved implements desktop.Hoverable.
func (c *CropOverlay) MouseMoved(_ *desktop.MouseEvent) {}

// --- coordinate helpers ---

// letterbox computes scale (display units per image pixel), offX, offY
// (top-left of the image in display space) for fitting img inside dispW×dispH.
func letterbox(dispW, dispH float32, img image.Image) (scale, offX, offY float32) {
	if img == nil || dispW <= 0 || dispH <= 0 {
		return 1, 0, 0
	}
	b := img.Bounds()
	iW := float32(b.Dx())
	iH := float32(b.Dy())
	sx := dispW / iW
	sy := dispH / iH
	if sx < sy {
		scale = sx
	} else {
		scale = sy
	}
	offX = (dispW - iW*scale) / 2
	offY = (dispH - iH*scale) / 2
	return
}

func imgToDisp(p image.Point, scale, offX, offY float32) fyne.Position {
	return fyne.NewPos(float32(p.X)*scale+offX, float32(p.Y)*scale+offY)
}

func dispToImg(p fyne.Position, scale, offX, offY float32) image.Point {
	return image.Point{
		X: int((p.X - offX) / scale),
		Y: int((p.Y - offY) / scale),
	}
}

// --- handle positions ---

// handlePositions returns handle positions in display units.
// Axis-aligned: 8 handles (4 corners + 4 edge midpoints, indices 0–7).
// Free quad: 4 corner handles (indices 0–3).
//
// Corner/midpoint index map (axis-aligned):
//
//	0=TL  4=top  1=TR
//	7=left       5=right
//	3=BL  6=bot  2=BR
func (c *CropOverlay) handlePositions(scale, offX, offY float32) []fyne.Position {
	pts := c.cropPts
	if c.freeQuad {
		h := make([]fyne.Position, 4)
		for i, p := range pts {
			h[i] = imgToDisp(p, scale, offX, offY)
		}
		return h
	}
	minX, minY := pts[0].X, pts[0].Y
	maxX, maxY := pts[2].X, pts[2].Y
	midX := (minX + maxX) / 2
	midY := (minY + maxY) / 2
	imgPts := [8]image.Point{
		{X: minX, Y: minY}, {X: maxX, Y: minY}, {X: maxX, Y: maxY}, {X: minX, Y: maxY},
		{X: midX, Y: minY}, {X: maxX, Y: midY}, {X: midX, Y: maxY}, {X: minX, Y: midY},
	}
	h := make([]fyne.Position, 8)
	for i, p := range imgPts {
		h[i] = imgToDisp(p, scale, offX, offY)
	}
	return h
}

func (c *CropOverlay) hitTestHandle(pos fyne.Position) int {
	scale, offX, offY := letterbox(c.widgetSize.Width, c.widgetSize.Height, c.img)
	handles := c.handlePositions(scale, offX, offY)
	half := float32(handleHit) / 2
	for i, hp := range handles {
		if pos.X >= hp.X-half && pos.X <= hp.X+half &&
			pos.Y >= hp.Y-half && pos.Y <= hp.Y+half {
			return i
		}
	}
	return -1
}

// applyDrag updates cropPts based on the current mouse position.
func (c *CropOverlay) applyDrag(pos fyne.Position) {
	scale, offX, offY := letterbox(c.widgetSize.Width, c.widgetSize.Height, c.img)
	newPt := dispToImg(pos, scale, offX, offY)
	imgB := c.img.Bounds()

	if c.freeQuad {
		newPt.X = clampInt(newPt.X, imgB.Min.X, imgB.Max.X-1)
		newPt.Y = clampInt(newPt.Y, imgB.Min.Y, imgB.Max.Y-1)
		c.cropPts[c.activeHandle] = newPt
		return
	}

	minX := c.cropPts[0].X
	minY := c.cropPts[0].Y
	maxX := c.cropPts[2].X
	maxY := c.cropPts[2].Y
	// Minimum size in image pixels, derived from display minimum.
	minSz := int(minCropDisp/scale) + 1

	switch c.activeHandle {
	case 0: // TL corner
		minX = clampInt(newPt.X, imgB.Min.X, maxX-minSz)
		minY = clampInt(newPt.Y, imgB.Min.Y, maxY-minSz)
	case 1: // TR corner
		maxX = clampInt(newPt.X, minX+minSz, imgB.Max.X)
		minY = clampInt(newPt.Y, imgB.Min.Y, maxY-minSz)
	case 2: // BR corner
		maxX = clampInt(newPt.X, minX+minSz, imgB.Max.X)
		maxY = clampInt(newPt.Y, minY+minSz, imgB.Max.Y)
	case 3: // BL corner
		minX = clampInt(newPt.X, imgB.Min.X, maxX-minSz)
		maxY = clampInt(newPt.Y, minY+minSz, imgB.Max.Y)
	case 4: // top mid
		minY = clampInt(newPt.Y, imgB.Min.Y, maxY-minSz)
	case 5: // right mid
		maxX = clampInt(newPt.X, minX+minSz, imgB.Max.X)
	case 6: // bottom mid
		maxY = clampInt(newPt.Y, minY+minSz, imgB.Max.Y)
	case 7: // left mid
		minX = clampInt(newPt.X, imgB.Min.X, maxX-minSz)
	}

	c.cropPts = [4]image.Point{
		{X: minX, Y: minY},
		{X: maxX, Y: minY},
		{X: maxX, Y: maxY},
		{X: minX, Y: maxY},
	}
}

func clampInt(v, lo, hi int) int {
	if lo > hi {
		hi = lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// --- overlay raster generator ---

// generateOverlay produces the transparent overlay drawn on top of the bgImage:
// dim regions, crop border, handles, and the loupe when dragging.
// The scanned image itself is rendered by bgImage (canvas.Image, GPU-accelerated).
func (c *CropOverlay) generateOverlay(w, h int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	// dst starts fully transparent (all zeros).

	if c.img == nil {
		return dst
	}

	scale, offX, offY := letterbox(float32(w), float32(h), c.img)
	imgB := c.img.Bounds()
	dispW := int(float32(imgB.Dx()) * scale)
	dispH := int(float32(imgB.Dy()) * scale)
	iOffX := int(offX)
	iOffY := int(offY)

	// Crop quad in display (physical pixel) coords.
	var dispQuad [4]fyne.Position
	for i := range 4 {
		dispQuad[i] = imgToDisp(c.cropPts[i], scale, offX, offY)
	}

	// Dim the area outside the crop region.
	c.applyDimOverlay(dst, iOffX, iOffY, iOffX+dispW, iOffY+dispH, dispQuad)

	// White 2px crop border.
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	for i := range 4 {
		a := dispQuad[i]
		b := dispQuad[(i+1)%4]
		drawThickLine(dst, int(a.X), int(a.Y), int(b.X), int(b.Y), white, 2)
	}

	// Handles.
	handles := c.handlePositions(scale, offX, offY)
	dark := color.RGBA{R: 30, G: 30, B: 30, A: 255}
	hh := handleVisual / 2
	for _, hp := range handles {
		hx, hy := int(hp.X), int(hp.Y)
		fillRect(dst, image.Rect(hx-hh, hy-hh, hx+hh+1, hy+hh+1), white)
		drawRectBorder(dst, image.Rect(hx-hh, hy-hh, hx+hh+1, hy+hh+1), dark)
	}

	// Loupe when dragging a handle.
	if c.activeHandle >= 0 && c.activeHandle < len(handles) {
		c.drawLoupe(dst, w, h, handles[c.activeHandle], scale, offX, offY)
	}

	return dst
}

// applyDimOverlay paints semi-transparent black over pixels outside the crop quad.
// imgX0,imgY0,imgX1,imgY1 are the pixel bounds of the letterboxed image area.
func (c *CropOverlay) applyDimOverlay(dst *image.RGBA, imgX0, imgY0, imgX1, imgY1 int, quad [4]fyne.Position) {
	if !c.freeQuad {
		// Axis-aligned fast path: four dim rectangles surrounding the crop box.
		cx0 := int(quad[0].X)
		cy0 := int(quad[0].Y)
		cx1 := int(quad[2].X)
		cy1 := int(quad[2].Y)
		dimRegion(dst, image.Rect(imgX0, imgY0, imgX1, cy0))
		dimRegion(dst, image.Rect(imgX0, cy1, imgX1, imgY1))
		dimRegion(dst, image.Rect(imgX0, cy0, cx0, cy1))
		dimRegion(dst, image.Rect(cx1, cy0, imgX1, cy1))
		return
	}
	// Free quad: per-pixel point-in-polygon test within the image area.
	for y := imgY0; y < imgY1; y++ {
		for x := imgX0; x < imgX1; x++ {
			fp := fyne.NewPos(float32(x)+0.5, float32(y)+0.5)
			if !pointInQuad(quad, fp) {
				dst.SetRGBA(x, y, color.RGBA{A: 165})
			}
		}
	}
}

// dimRegion fills r with semi-transparent black using a single draw call.
func dimRegion(dst *image.RGBA, r image.Rectangle) {
	r = r.Intersect(dst.Bounds())
	draw.Draw(dst, r, dimColor, image.Point{}, draw.Src)
}

// pointInQuad returns true if p is inside the convex quad (TL,TR,BR,BL, clockwise).
func pointInQuad(pts [4]fyne.Position, p fyne.Position) bool {
	for i := range 4 {
		a := pts[i]
		b := pts[(i+1)%4]
		// For clockwise winding (Y-down), cross >= 0 means inside.
		cross := (b.X-a.X)*(p.Y-a.Y) - (b.Y-a.Y)*(p.X-a.X)
		if cross < 0 {
			return false
		}
	}
	return true
}

func (c *CropOverlay) drawLoupe(dst *image.RGBA, w, h int, handlePos fyne.Position, scale, offX, offY float32) {
	// Place loupe in the corner farthest from the active handle.
	var lx, ly int
	if handlePos.X < float32(w)/2 {
		lx = w - loupeOutSize - 8
	} else {
		lx = 8
	}
	if handlePos.Y < float32(h)/2 {
		ly = h - loupeOutSize - 8
	} else {
		ly = 8
	}

	// Source region: loupeSrcSize×loupeSrcSize centred on handle in image coords.
	ctr := dispToImg(handlePos, scale, offX, offY)
	imgB := c.img.Bounds()
	sx0 := clampInt(ctr.X-loupeSrcSize/2, imgB.Min.X, imgB.Max.X-loupeSrcSize)
	sy0 := clampInt(ctr.Y-loupeSrcSize/2, imgB.Min.Y, imgB.Max.Y-loupeSrcSize)

	// Scale up to loupeOutSize×loupeOutSize (nearest-neighbour).
	for py := 0; py < loupeOutSize; py++ {
		for px := 0; px < loupeOutSize; px++ {
			sx := clampInt(sx0+px*loupeSrcSize/loupeOutSize, imgB.Min.X, imgB.Max.X-1)
			sy := clampInt(sy0+py*loupeSrcSize/loupeOutSize, imgB.Min.Y, imgB.Max.Y-1)
			r, g, b, a := c.img.At(sx, sy).RGBA()
			dpx := image.Point{X: lx + px, Y: ly + py}
			if dpx.In(dst.Bounds()) {
				dst.SetRGBA(dpx.X, dpx.Y, color.RGBA{
					R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8),
				})
			}
		}
	}

	// White border around loupe.
	drawRectBorder(dst, image.Rect(lx, ly, lx+loupeOutSize, ly+loupeOutSize),
		color.RGBA{R: 255, G: 255, B: 255, A: 255})

	// Crosshair at loupe centre.
	cx, cy := lx+loupeOutSize/2, ly+loupeOutSize/2
	ch := color.RGBA{R: 255, G: 255, B: 255, A: 200}
	for i := -12; i <= 12; i++ {
		if i != 0 {
			setPixelSafe(dst, cx+i, cy, ch)
			setPixelSafe(dst, cx, cy+i, ch)
		}
	}
}

// --- drawing primitives ---

func fillRect(dst *image.RGBA, r image.Rectangle, c color.RGBA) {
	r = r.Intersect(dst.Bounds())
	draw.Draw(dst, r, image.NewUniform(c), image.Point{}, draw.Src)
}

func drawRectBorder(dst *image.RGBA, r image.Rectangle, c color.RGBA) {
	for x := r.Min.X; x < r.Max.X; x++ {
		setPixelSafe(dst, x, r.Min.Y, c)
		setPixelSafe(dst, x, r.Max.Y-1, c)
	}
	for y := r.Min.Y + 1; y < r.Max.Y-1; y++ {
		setPixelSafe(dst, r.Min.X, y, c)
		setPixelSafe(dst, r.Max.X-1, y, c)
	}
}

func setPixelSafe(dst *image.RGBA, x, y int, c color.RGBA) {
	if image.Pt(x, y).In(dst.Bounds()) {
		dst.SetRGBA(x, y, c)
	}
}

func drawThickLine(dst *image.RGBA, x0, y0, x1, y1 int, c color.RGBA, thickness int) {
	adx := x1 - x0
	if adx < 0 {
		adx = -adx
	}
	ady := y1 - y0
	if ady < 0 {
		ady = -ady
	}
	half := thickness / 2
	for t := -half; t < thickness-half; t++ {
		if adx >= ady {
			drawLine(dst, x0, y0+t, x1, y1+t, c)
		} else {
			drawLine(dst, x0+t, y0, x1+t, y1, c)
		}
	}
}

func drawLine(dst *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	adx := x1 - x0
	if adx < 0 {
		adx = -adx
	}
	ady := y1 - y0
	if ady < 0 {
		ady = -ady
	}
	sx := 1
	if x0 > x1 {
		sx = -1
	}
	sy := 1
	if y0 > y1 {
		sy = -1
	}
	err := adx - ady
	for {
		setPixelSafe(dst, x0, y0, c)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -ady {
			err -= ady
			x0 += sx
		}
		if e2 < adx {
			err += adx
			y0 += sy
		}
	}
}

// --- renderer ---

type cropRenderer struct {
	overlay     *CropOverlay
	bgImage     *canvas.Image
	raster      *canvas.Raster
	logoImage   *canvas.Image
	placeholder *canvas.Text
}

func (r *cropRenderer) Layout(size fyne.Size) {
	r.overlay.widgetSize = size
	r.bgImage.Resize(size)
	r.raster.Resize(size)

	// Centre the logo + text block vertically.
	blockH := float32(placeholderLogoH + placeholderGap + placeholderTextH)
	blockY := (size.Height - blockH) / 2
	r.logoImage.Resize(fyne.NewSize(placeholderLogoW, placeholderLogoH))
	r.logoImage.Move(fyne.NewPos((size.Width-placeholderLogoW)/2, blockY))
	r.placeholder.Resize(fyne.NewSize(size.Width, placeholderTextH))
	r.placeholder.Move(fyne.NewPos(0, blockY+placeholderLogoH+placeholderGap))
}

func (r *cropRenderer) MinSize() fyne.Size {
	return fyne.NewSize(100, 100)
}

func (r *cropRenderer) Refresh() {
	if r.overlay.img == nil {
		r.bgImage.Hide()
		r.logoImage.Show()
		r.placeholder.Text = r.overlay.placeholderText
		r.placeholder.Show()
	} else {
		r.bgImage.Image = r.overlay.img
		r.bgImage.Show()
		r.logoImage.Hide()
		r.placeholder.Hide()
	}
	r.bgImage.Refresh()
	r.raster.Refresh()
	r.logoImage.Refresh()
	r.placeholder.Refresh()
}

func (r *cropRenderer) Destroy() {}

// Objects returns canvas objects bottom-to-top: bgImage, overlay raster, logo, placeholder.
func (r *cropRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bgImage, r.raster, r.logoImage, r.placeholder}
}
