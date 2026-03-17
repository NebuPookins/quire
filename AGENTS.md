# Quire — Implementation Plan for Coding Agents

This file breaks the project into discrete, ordered steps. Each step is self-contained
and testable. Later steps depend on earlier ones. Follow the spec in `scanner-spec.md`
for all behavioral details.

---

## Step 1 — Project Scaffolding ✅ DONE

**Resolved versions:** `fyne.io/fyne/v2 v2.7.3`, `gocv.io/x/gocv v0.43.0`

Files created:
- `go.mod` / `go.sum` — module `quire`, deps added + tidied
- `main.go` — minimal Fyne window (title "Quire", 800×600), `go build ./...` passes
- `config/config.go` — stub `Config`, `Load()`, `Save()`
- `scanner/scanner.go` — stub `ListDevices()`, `Scan()`, `ErrScanImageNotFound`
- `detect/edges.go` — stub `DetectQuad()`, tunable constants
- `ui/cropoverlay.go` — stub `CropOverlay` widget skeleton
- `ui/mainwindow.go` — stub `MainWindow`, `AppState` type, `setState()`
- `export/jpeg.go` — stub `SaveAxisAligned()`, `SavePerspective()`

All stubs compile; unimplemented functions panic at runtime (intentional).

---

## Step 2 — Config Persistence ✅ DONE

Files: `config/config.go`, `config/config_test.go`

- `configPath()` resolves `$XDG_CONFIG_HOME/quire/config.json`, falling back to
  `~/.config/quire/config.json`.
- `Load()` — reads and unmarshals JSON; returns defaults on any error (missing file,
  malformed JSON, empty `LastSaveDir`). Never crashes.
- `Save()` — creates parent dirs, writes to a temp file in the same directory, renames
  atomically.
- 5 unit tests, all passing (`go test ./config/... -v`).

---

## Step 3 — Scanner Discovery & Subprocess ✅ DONE

Files: `scanner/scanner.go`, `scanner/scanner_test.go`

- `parseDevices(output string) []string` — extracts device names from `scanimage -L`
  output using a regex; tested directly without exec.
- `ListDevices()` — runs `scanimage -L`, returns `ErrScanImageNotFound` if not on PATH.
- `Scan()` — runs `scanimage --device ... --format=pnm --mode ... --resolution ...`;
  returns error on non-zero exit or any stderr output.
- `decodePNM()` — minimal built-in decoder for P4 (PBM/Lineart), P5 (PGM/Gray),
  P6 (PPM/Color), including comment-line support. No extra dependency needed.
- 10 unit tests, all passing (`go test ./scanner/... -v`).

---

## Step 4 — Edge Detection (`detect/edges.go`)

- Tunable constants at top of file:
  ```go
  const (
      CannyLow       = 75
      CannyHigh      = 200
      LoupeSourceSize = 40
  )
  ```
- `DetectQuad(img image.Image) [4]image.Point` — implements the pipeline:
  1. Convert `image.Image` → `gocv.Mat` (RGBA→BGR).
  2. Grayscale → Gaussian blur (5×5) → Canny.
  3. `FindContours` (external only).
  4. For each contour: convex hull → `ApproxPolyDP` (ε = 0.02 × arc length).
  5. Keep 4-vertex polygons; pick the largest by area.
  6. Fallback: full image bounding rect.
  7. Order corners: top-left, top-right, bottom-right, bottom-left.
  8. Return as `[4]image.Point`.
  - `defer mat.Close()` at every `gocv.Mat` allocation.
- Unit-test with a synthetic white rectangle on black background — detection should
  return approximately the rectangle corners.

---

## Step 5 — Export (`export/jpeg.go`)

- `SaveAxisAligned(img image.Image, topLeft, bottomRight image.Point, path string) error`
  — crops the sub-image and encodes as JPEG quality 92.
- `SavePerspective(img image.Image, quad [4]image.Point, path string) error`
  — uses `gocv.GetPerspectiveTransform` + `gocv.WarpPerspective` to produce a flat
  rectangular output. Output size = bounding rect of the four points. Encodes JPEG 92.
- Both functions write to a temp file and rename (atomic write).
- Unit-test axis-aligned crop with a known image and verify output dimensions.

---

## Step 6 — App State & Main Window Skeleton (`ui/mainwindow.go`)

Define the central state struct (no global variables):

```go
type AppState int
const (
    StateIdle AppState = iota
    StateScanning
    StateReady
    StateSaving
)

type MainWindow struct {
    window       fyne.Window
    app          fyne.App
    state        AppState
    scannedImage image.Image
    detectedQuad [4]image.Point
    config       config.Config
    // UI widgets stored as fields so setState can toggle them
    scanBtn      *widget.Button
    saveBtn      *widget.Button
    resetBtn     *widget.Button
    freeQuadChk  *widget.Check
    deviceSelect *widget.Select
    resSelect    *widget.Select
    modeSelect   *widget.Select
    cropOverlay  *CropOverlay  // defined in Step 7
}
```

- `NewMainWindow(a fyne.App) *MainWindow` — constructs the window, builds the toolbar
  and bottom bar (with placeholder widgets), sets min size 800×600.
- `setState(s AppState)` — enables/disables buttons per the F7 state table. Must run
  on the Fyne main thread (called from UI handlers, safe; if called from goroutines use
  `fyne.Do()`).
- Wire up scanner discovery on startup:
  - Run `scanner.ListDevices()`.
  - Zero devices → error dialog, Scan button disabled.
  - One device → auto-select.
  - Many devices → populate `deviceSelect` dropdown.
  - `scanimage` not found → error dialog, Scan button disabled.
- Wire up the **Scan** button handler (goroutine):
  1. `setState(StateScanning)`, show progress indicator.
  2. Call `scanner.Scan(...)`.
  3. On error: `fyne.Do(func(){ error dialog; setState(StateIdle) })`.
  4. On success: `fyne.Do(func(){ store image; run DetectQuad; store detectedQuad;
     update CropOverlay; setState(StateReady) })`.
- Wire up the **Save** button handler (Step 8 fills in export logic).
- Wire up the **Reset Crop** button handler: restore crop to `detectedQuad` (or its
  axis-aligned bounding rect per current mode).

---

## Step 7 — CropOverlay Widget (`ui/cropoverlay.go`)

This is the most complex piece. Build it incrementally:

### 7a — Basic image display
- `CropOverlay` embeds `widget.BaseWidget`.
- `Renderer` draws the scanned image scaled to fit (letterboxed, aspect-ratio preserved).
- Before a scan: render placeholder text "Press Scan to begin."
- Expose `SetImage(img image.Image)` to update and `Refresh()`.

### 7b — Crop box rendering
- Internal state: `cropPts [4]image.Point` in image coordinates; `freeQuad bool`.
- `SetCrop(pts [4]image.Point, freeQuad bool)` — update state + Refresh.
- Renderer draws:
  - Dark semi-transparent fill outside crop region.
  - White 2px border around crop region.
  - Handles: 12×12 px white squares with dark border.
    - Axis-aligned: 8 handles (4 corners + 4 edge midpoints).
    - Free quad: 4 corner handles.
- Coordinate helpers: `imageToDisplay(p image.Point) fyne.Position` and
  `displayToImage(p fyne.Position) image.Point` using the current letterbox
  scale + offset.

### 7c — Mouse interaction
- Implement `desktop.Mouseable` (`MouseDown`, `MouseUp`, `MouseMoved`) and
  `desktop.Hoverable` (`MouseIn`, `MouseOut`, `MouseMoved`).
- `MouseDown`: hit-test handles (24×24 px hit target), set `activeHandle` index.
- `MouseMoved` while dragging:
  - Update the relevant `cropPts` entry (or two for edge-midpoint handles).
  - Enforce minimum 20px width/height in display coordinates.
  - Axis-aligned: after updating a corner, re-derive the other three from the
    min/max of all four to keep rect aligned.
  - Call `Refresh()`.
- `MouseUp`: clear `activeHandle`, clear loupe, `Refresh()`.

### 7d — Loupe overlay
- While `activeHandle >= 0`, composite a 200×200 loupe into the widget's draw pass.
- Source region: `LoupeSourceSize × LoupeSourceSize` pixels from the full-resolution
  image centered on the active handle's image-coordinate position. Clamp to image bounds.
- Scale that region up to 200×200.
- Draw a crosshair in the center.
- Position loupe in the farthest corner from the active handle (compare handle's
  display position to widget center).
- Use `canvas.Raster` or paint via `canvas.NewRasterWithPixels` — no separate OS window.

### 7e — Free quad toggle
- `SetFreeQuad(fq bool)` — called by the bottom-bar checkbox.
- On `false → true`: convert current rect's two stored points into 4 explicit corners,
  no visual change.
- On `true → false`: snap current quad to its axis-aligned bounding rect.
- Update renderer handle set accordingly.

---

## Step 8 — Save Handler (complete `ui/mainwindow.go`)

- **Save** button: `setState(StateSaving)`, open `dialog.ShowFileSave`.
  - Initial directory: `cfg.LastSaveDir`.
  - Default filename: `scan.jpg`.
  - On user confirmation:
    1. Goroutine: call `export.SaveAxisAligned` or `export.SavePerspective` based on
       current mode.
    2. `fyne.Do`: on success → update `cfg.LastSaveDir`, call `config.Save`, show
       success notification, `setState(StateReady)`.
    3. `fyne.Do`: on error → error dialog, `setState(StateReady)`.
  - On dialog cancel: `setState(StateReady)`.

---

## Step 9 — Integration & Polish

- Verify the F7 state table: manually trace all transitions; write a table-driven test
  for `setState` if feasible.
- Ensure every goroutine that touches a Fyne widget does so via `fyne.Do()`.
- Audit all `gocv.Mat` allocations for matching `.Close()` / `defer mat.Close()`.
- Review error paths:
  - `scanimage` not on PATH (startup).
  - Scan subprocess error.
  - OpenCV no-quad fallback (silent).
  - File write failure.
  - Config unreadable (silent, use defaults).
- Confirm window resizing works: `CropOverlay` fills available space; letterbox
  recalculates on `Resize()`.
- Set final window title to "Quire".

---

## Step 10 — Build Verification

- `go build ./...` must produce zero errors and zero warnings.
- `go vet ./...` must pass.
- Run unit tests: `go test ./...`.
- Manual smoke test checklist:
  - [ ] App starts; scanner discovery runs.
  - [ ] Scan button triggers scan; spinner visible; image appears after scan.
  - [ ] Auto-detected crop box is visible.
  - [ ] Drag each handle type; loupe appears/disappears correctly.
  - [ ] Free quad toggle switches modes; Reset Crop restores quad.
  - [ ] Save dialog opens in last-save dir; JPEG written; success notification shown.
  - [ ] Config file updated after save.
  - [ ] Resize window; crop overlay reflows correctly.

---

## Implementation Order Summary

| # | File(s) | Deliverable |
|---|---------|-------------|
| 1 ✅ | `go.mod`, stubs, `main.go` | Compiles, blank window |
| 2 ✅ | `config/config.go` | Persist/load last-save dir |
| 3 ✅ | `scanner/scanner.go` | Device list, scan subprocess |
| 4 | `detect/edges.go` | OpenCV quad detection |
| 5 | `export/jpeg.go` | Axis-aligned + perspective JPEG export |
| 6 | `ui/mainwindow.go` | State machine, toolbar, button wiring |
| 7 | `ui/cropoverlay.go` | Custom widget (image, handles, loupe) |
| 8 | `ui/mainwindow.go` | Save handler wired to export + config |
| 9 | All | Polish, error paths, thread-safety audit |
| 10 | — | Build + smoke test |
