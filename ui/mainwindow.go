package ui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"path/filepath"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/ncruces/zenity"

	"quire/config"
	"quire/detect"
	"quire/export"
	"quire/scanner"
)

// AppState represents the top-level application state.
type AppState int

const (
	StateBootingUp         AppState = iota // device discovery in progress
	StateWaitingForDevice                  // devices listed; user must pick one
	StateWaitingForOptions                 // device selected; querying its capabilities
	StateIdle                              // device ready; no scan yet
	StateScanning                          // scan in progress
	StateReady                             // scan acquired, ready to edit/save
	StateSaving                            // save in progress
)

// MainWindow holds all application state and top-level UI references.
type MainWindow struct {
	window fyne.Window
	app    fyne.App
	state  AppState

	scannedImage   image.Image
	detectedQuad   *[4]image.Point // nil before the first scan; full bounds thereafter (Reset Crop target)
	cfg            config.Config
	devices        []scanner.Device
	selectedDevice scanner.Device

	scanBtn     *widget.Button
	saveBtn     *widget.Button
	resetBtn    *widget.Button
	freeQuadChk *widget.Check
	deviceSel   *widget.Select
	resSel      *widget.Select
	modeSel     *widget.Select
	spinner     *widget.Activity
	progressBar *widget.ProgressBar
	cropOverlay *CropOverlay
}

// NewMainWindow constructs the main application window and starts scanner
// discovery. Call Show() to display the window and enter the Fyne event loop.
func NewMainWindow(a fyne.App) *MainWindow {
	mw := &MainWindow{app: a, cfg: config.Load()}
	mw.window = a.NewWindow("Quire")

	mw.scanBtn = widget.NewButton("Scan", mw.onScan)
	mw.saveBtn = widget.NewButton("Save", mw.onSave)
	mw.resetBtn = widget.NewButton("Reset Crop", mw.onResetCrop)
	mw.freeQuadChk = widget.NewCheck("Free quad", mw.onFreeQuadToggle)
	mw.deviceSel = widget.NewSelect(nil, mw.onDeviceSelected)
	mw.resSel = widget.NewSelect(nil, nil)
	mw.modeSel = widget.NewSelect(nil, nil)
	mw.spinner = widget.NewActivity()
	mw.progressBar = widget.NewProgressBar()
	mw.cropOverlay = NewCropOverlay()

	mw.setState(StateBootingUp)
	mw.spinner.Hide()
	mw.progressBar.Hide()

	toolbar := container.NewHBox(mw.deviceSel, mw.resSel, mw.modeSel)
	leftBar := container.NewHBox(mw.resetBtn, mw.freeQuadChk)
	rightBar := container.NewHBox(mw.spinner, mw.progressBar, mw.scanBtn, mw.saveBtn)
	bottomBar := container.NewBorder(nil, nil, leftBar, rightBar)
	content := container.NewBorder(toolbar, bottomBar, nil, nil, mw.cropOverlay)

	mw.window.SetContent(content)
	mw.window.Resize(fyne.NewSize(800, 600))

	go mw.discoverDevices()

	return mw
}

// Show displays the window and enters the Fyne event loop. It blocks until
// the window is closed.
func (mw *MainWindow) Show() {
	mw.window.ShowAndRun()
}

// setState enables/disables UI controls per the F7 state table.
// Safe to call from the UI goroutine; use fyne.Do when calling from other goroutines.
func (mw *MainWindow) setState(s AppState) {
	mw.state = s
	switch s {
	case StateBootingUp:
		mw.cropOverlay.SetPlaceholder("Please wait, detecting scanners…")
		mw.deviceSel.Disable()
		mw.resSel.Hide()
		mw.modeSel.Hide()
		mw.scanBtn.Disable()
		mw.saveBtn.Disable()
		mw.resetBtn.Disable()
		mw.freeQuadChk.Disable()
	case StateWaitingForDevice:
		mw.cropOverlay.SetPlaceholder("Select a scanner from the Device dropdown.")
		mw.deviceSel.Enable()
		mw.resSel.Hide()
		mw.modeSel.Hide()
		mw.scanBtn.Disable()
		mw.saveBtn.Disable()
		mw.resetBtn.Disable()
		mw.freeQuadChk.Disable()
	case StateWaitingForOptions:
		mw.cropOverlay.SetPlaceholder("Detecting scanner options…")
		mw.deviceSel.Disable()
		mw.resSel.Hide()
		mw.modeSel.Hide()
		mw.scanBtn.Disable()
		mw.saveBtn.Disable()
		mw.resetBtn.Disable()
		mw.freeQuadChk.Disable()
	case StateIdle:
		mw.cropOverlay.SetPlaceholder("Press Scan to begin.")
		mw.deviceSel.Enable()
		mw.resSel.Enable()
		mw.modeSel.Enable()
		mw.scanBtn.Enable()
		mw.saveBtn.Disable()
		mw.resetBtn.Disable()
		mw.freeQuadChk.Disable()
	case StateScanning:
		mw.cropOverlay.SetPlaceholder("Please wait… scanning.")
		mw.deviceSel.Disable()
		mw.resSel.Disable()
		mw.modeSel.Disable()
		mw.scanBtn.Disable()
		mw.saveBtn.Disable()
		mw.resetBtn.Disable()
		mw.freeQuadChk.Disable()
	case StateReady:
		mw.deviceSel.Enable()
		mw.resSel.Enable()
		mw.modeSel.Enable()
		mw.scanBtn.Enable()
		mw.saveBtn.Enable()
		mw.resetBtn.Enable()
		mw.freeQuadChk.Enable()
	case StateSaving:
		mw.deviceSel.Disable()
		mw.resSel.Disable()
		mw.modeSel.Disable()
		mw.scanBtn.Disable()
		mw.saveBtn.Disable()
		mw.resetBtn.Disable()
		mw.freeQuadChk.Disable()
	}
}

// discoverDevices runs scanimage device discovery and updates the UI.
// Must be called in a goroutine.
func (mw *MainWindow) discoverDevices() {
	devices, err := scanner.ListDevices()
	fyne.Do(func() {
		if err != nil {
			mw.setState(StateWaitingForDevice)
			if errors.Is(err, scanner.ErrScanImageNotFound) {
				dialog.ShowError(fmt.Errorf("scanimage not found on PATH — install SANE to use Quire"), mw.window)
			} else {
				dialog.ShowError(fmt.Errorf("scanner discovery failed: %w", err), mw.window)
			}
			return
		}
		if len(devices) == 0 {
			mw.setState(StateWaitingForDevice)
			dialog.ShowInformation("No scanners found",
				"No scanners were detected by SANE.\nConnect a scanner and restart.", mw.window)
			return
		}
		mw.devices = devices
		descs := make([]string, len(devices))
		for i, d := range devices {
			descs[i] = d.Description
		}
		mw.deviceSel.Options = descs
		mw.deviceSel.Refresh()
		// Auto-select: prefer the last-used device, fall back to the only device.
		autoSelect := ""
		for _, d := range devices {
			if d.Name == mw.cfg.LastDevice {
				autoSelect = d.Description
				break
			}
		}
		if autoSelect == "" && len(devices) == 1 {
			autoSelect = descs[0]
		}
		if autoSelect != "" {
			// onDeviceSelected fires via SetSelected and transitions to StateWaitingForOptions.
			mw.deviceSel.SetSelected(autoSelect)
		} else {
			mw.setState(StateWaitingForDevice)
		}
	})
}

// onDeviceSelected is called when the user picks a device from the dropdown.
func (mw *MainWindow) onDeviceSelected(desc string) {
	for _, d := range mw.devices {
		if d.Description == desc {
			mw.selectedDevice = d
			break
		}
	}
	mw.cfg.LastDevice = mw.selectedDevice.Name
	config.Save(mw.cfg) //nolint:errcheck — non-critical
	mw.setState(StateWaitingForOptions)
	go mw.queryDeviceOptions()
}

// queryDeviceOptions fetches the mode/resolution options for the selected device.
// Must be called in a goroutine.
func (mw *MainWindow) queryDeviceOptions() {
	opts, err := scanner.QueryOptions(mw.selectedDevice.Name)
	fyne.Do(func() {
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to query device options: %w", err), mw.window)
			mw.setState(StateWaitingForDevice)
			return
		}

		if len(opts.Resolutions) == 0 {
			mw.resSel.Hide()
		} else {
			resStrs := make([]string, len(opts.Resolutions))
			for i, r := range opts.Resolutions {
				resStrs[i] = strconv.Itoa(r)
			}
			mw.resSel.Options = resStrs
			mw.resSel.SetSelected(strconv.Itoa(closestResolution(opts.Resolutions, 300)))
			mw.resSel.Show()
			mw.resSel.Refresh()
		}

		if len(opts.Modes) == 0 {
			mw.modeSel.Hide()
		} else {
			modeStrs := make([]string, len(opts.Modes))
			for i, m := range opts.Modes {
				modeStrs[i] = string(m)
			}
			mw.modeSel.Options = modeStrs
			// Prefer "Color"; fall back to the device's advertised default; then first option.
			sel := preferredMode(modeStrs, string(opts.DefaultMode))
			mw.modeSel.SetSelected(sel)
			mw.modeSel.Show()
			mw.modeSel.Refresh()
		}

		if mw.scannedImage != nil {
			mw.setState(StateReady)
		} else {
			mw.setState(StateIdle)
		}
	})
}

// onScan handles the Scan button. Runs the scan in a goroutine.
func (mw *MainWindow) onScan() {
	mw.setState(StateScanning)
	mw.spinner.Show()
	mw.spinner.Start()
	mw.progressBar.SetValue(0)
	mw.progressBar.Show()

	dev := mw.selectedDevice

	var mode *scanner.Mode
	if sel := mw.modeSel.Selected; sel != "" {
		m := scanner.Mode(sel)
		mode = &m
	}
	var resolution *int
	if sel := mw.resSel.Selected; sel != "" {
		if r, err := strconv.Atoi(sel); err == nil {
			resolution = &r
		}
	}

	go func() {
		progressFn := func(pct float64) {
			fyne.Do(func() { mw.progressBar.SetValue(pct) })
		}
		img, scanErr := scanner.Scan(context.Background(), dev.Name, mode, resolution, progressFn)
		fyne.Do(func() {
			mw.spinner.Stop()
			mw.spinner.Hide()
			mw.progressBar.Hide()
			mw.progressBar.SetValue(0)
			if scanErr != nil {
				dialog.ShowError(fmt.Errorf("scan failed: %w", scanErr), mw.window)
				mw.setState(StateIdle)
				return
			}
			mw.applyScanResult(img)
		})
	}()
}

// applyScanResult updates the UI after a successful scan.
// On the first scan detectedQuad is nil, so the crop defaults to the full image
// bounds. On subsequent scans detectedQuad is already set and left untouched;
// the overlay's existing crop (whatever the user last set) is reused instead,
// clamped to the new image bounds in case the resolution changed.
func (mw *MainWindow) applyScanResult(img image.Image) {
	mw.scannedImage = img

	var quad [4]image.Point
	if mw.detectedQuad == nil {
		q := detect.DetectQuad(img)
		mw.detectedQuad = &q
		quad = q
	} else {
		quad = clampQuad(mw.cropOverlay.CurrentCrop(), img.Bounds())
	}

	if !mw.freeQuadChk.Checked {
		quad = axisAlignedQuad(quad)
	}
	mw.cropOverlay.SetImage(img)
	mw.cropOverlay.SetCrop(quad, mw.freeQuadChk.Checked)
	mw.setState(StateReady)
}

// onSave handles the Save button. Opens the native system file-save dialog in
// a goroutine (zenity is blocking) and posts UI updates back via fyne.Do.
func (mw *MainWindow) onSave() {
	mw.setState(StateSaving)

	img := mw.scannedImage
	quad := mw.cropOverlay.CurrentCrop()
	freeQuad := mw.freeQuadChk.Checked
	initialPath := filepath.Join(mw.cfg.LastSaveDir, "scan.jpg")

	go func() {
		path, err := zenity.SelectFileSave(
			zenity.Filename(initialPath),
			zenity.FileFilter{Name: "JPEG image", Patterns: []string{"*.jpg", "*.jpeg"}},
		)
		if err == zenity.ErrCanceled {
			fyne.Do(func() { mw.setState(StateReady) })
			return
		}
		if err != nil {
			fyne.Do(func() {
				dialog.ShowError(fmt.Errorf("file dialog error: %w", err), mw.window)
				mw.setState(StateReady)
			})
			return
		}
		// Append .jpg if the user didn't type an extension.
		if filepath.Ext(path) == "" {
			path += ".jpg"
		}

		var saveErr error
		if freeQuad {
			saveErr = export.SavePerspective(img, quad, path)
		} else {
			saveErr = export.SaveAxisAligned(img, quad[0], quad[2], path)
		}
		fyne.Do(func() {
			if saveErr != nil {
				dialog.ShowError(fmt.Errorf("save failed: %w", saveErr), mw.window)
				mw.setState(StateReady)
				return
			}
			mw.cfg.LastSaveDir = filepath.Dir(path)
			config.Save(mw.cfg) //nolint:errcheck — non-critical
			mw.app.SendNotification(fyne.NewNotification("Quire", "Saved "+path))
			mw.setState(StateReady)
		})
	}()
}

// onResetCrop restores the crop box to the full image bounds recorded on the
// first scan.
func (mw *MainWindow) onResetCrop() {
	if mw.detectedQuad == nil {
		return
	}
	if mw.freeQuadChk.Checked {
		mw.cropOverlay.SetCrop(*mw.detectedQuad, true)
	} else {
		mw.cropOverlay.SetCrop(axisAlignedQuad(*mw.detectedQuad), false)
	}
}

// onFreeQuadToggle handles the Free quad checkbox.
func (mw *MainWindow) onFreeQuadToggle(checked bool) {
	if mw.scannedImage == nil {
		return
	}
	pts := mw.cropOverlay.CurrentCrop()
	if checked {
		mw.cropOverlay.SetCrop(pts, true)
	} else {
		mw.cropOverlay.SetCrop(axisAlignedQuad(pts), false)
	}
}

// clampQuad clamps each point in pts to the given rectangle, so that a quad
// from a previous scan remains valid when the new image is a different size.
func clampQuad(pts [4]image.Point, r image.Rectangle) [4]image.Point {
	clamp := func(p image.Point) image.Point {
		if p.X < r.Min.X {
			p.X = r.Min.X
		} else if p.X > r.Max.X {
			p.X = r.Max.X
		}
		if p.Y < r.Min.Y {
			p.Y = r.Min.Y
		} else if p.Y > r.Max.Y {
			p.Y = r.Max.Y
		}
		return p
	}
	return [4]image.Point{clamp(pts[0]), clamp(pts[1]), clamp(pts[2]), clamp(pts[3])}
}

// axisAlignedQuad returns the axis-aligned bounding rect of pts as 4 corners
// ordered TL, TR, BR, BL.
func axisAlignedQuad(pts [4]image.Point) [4]image.Point {
	minX, minY := pts[0].X, pts[0].Y
	maxX, maxY := pts[0].X, pts[0].Y
	for _, p := range pts[1:] {
		if p.X < minX {
			minX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	return [4]image.Point{
		{X: minX, Y: minY},
		{X: maxX, Y: minY},
		{X: maxX, Y: maxY},
		{X: minX, Y: maxY},
	}
}

// closestResolution returns the element of options whose value is closest to target.
func closestResolution(options []int, target int) int {
	best := options[0]
	bestDiff := abs(options[0] - target)
	for _, o := range options[1:] {
		if d := abs(o - target); d < bestDiff {
			bestDiff = d
			best = o
		}
	}
	return best
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// preferredMode returns "Color" if present, then deviceDefault if present, then options[0].
func preferredMode(options []string, deviceDefault string) string {
	for _, o := range options {
		if o == "Color" {
			return o
		}
	}
	for _, o := range options {
		if o == deviceDefault {
			return o
		}
	}
	return options[0]
}
