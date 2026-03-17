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

## Step 4 — Edge Detection ✅ DONE

Files: `detect/edges.go`, `detect/cgo_flags.go`, `detect/edges_test.go`

- `DetectQuad` pipeline: `ImageToMatRGB` → grayscale → Gaussian blur (5×5) → Canny
  → `FindContours` (external) → per-contour: convex hull (index approach) →
  `ApproxPolyDP` (ε = 2% arc length) → pick largest 4-vertex polygon → `orderQuad`.
  Falls back to full image bounding rect if no quad is found.
- `processContour` is a helper function (uses function-scoped defers) to avoid the
  defer-in-loop problem.
- `orderQuad` sorts by Y then X to produce TL, TR, BR, BL order.
- `detect/cgo_flags.go` adds `-Wl,--as-needed` to prevent `libopencv_viz` (and its
  uninstalled VTK dependency) from being linked on this system.
- 4 unit tests, all passing (`go test ./detect/... -v`).
- Note: `gocv` API uses `RetrievalExternal` (not `RetrExternal`) and `ConvexHull`
  returns an error in v0.43.0.

---

## Step 5 — Export ✅ DONE

Files: `export/jpeg.go`, `export/cgo_flags.go`, `export/jpeg_test.go`

- `SaveAxisAligned` — crops via `SubImage` (with pixel-copy fallback), encodes JPEG 92, atomic write.
- `SavePerspective` — computes output size as `max(TR.X−TL.X, BR.X−BL.X)` × `max(BL.Y−TL.Y, BR.Y−TR.Y)`,
  uses `gocv.GetPerspectiveTransform` + `gocv.WarpPerspective`, converts warped Mat back
  via `mat.ToImage()`, then encodes JPEG 92. Returns error for degenerate quads.
- `writeJPEG` — shared atomic-write helper (temp file + rename).
- `cgo_flags.go` — `-Wl,--as-needed` to avoid linking libopencv_viz (same as detect).
- 5 unit tests, all passing (`go test ./export/... -v`).

---

## Step 6 — App State & Main Window Skeleton ✅ DONE

Files: `ui/mainwindow.go`, `ui/cropoverlay.go` (minimal renderer), `main.go`

- Full `MainWindow` struct with `scannedImage`, `detectedQuad`, `cfg`, `devices`,
  `selectedDevice`, all widget fields, and a `*widget.Activity` spinner.
- `NewMainWindow` builds the toolbar (device/res/mode selects), bottom bar
  (Reset Crop + Free quad on left; spinner + Scan + Save on right), and
  `container.NewBorder` layout. Starts `discoverDevices` goroutine.
- `Show()` calls `window.ShowAndRun()`.
- `setState` implements the F7 table; in `StateIdle`, `scanBtn` is only enabled
  when a device is actually selected.
- `discoverDevices` → `fyne.Do`: populates device dropdown, auto-selects if one
  device, shows error dialog if none or `scanimage` missing.
- `onDeviceSelected` → `queryDeviceOptions` goroutine → `fyne.Do`: populates
  mode and resolution dropdowns from `scanner.QueryOptions`; defaults to "Color"
  and "300" when available.
- `onScan` goroutine: `setState(StateScanning)`, spinner start/show, calls
  `scanner.Scan`, on success stores image, runs `detect.DetectQuad`, updates
  `CropOverlay`, `setState(StateReady)`.
- `onResetCrop` and `onFreeQuadToggle` implemented; `onSave` is a stub for Step 8.
- `axisAlignedQuad` and `preferredOption` helpers in `mainwindow.go`.
- `CropOverlay.CreateRenderer` is a minimal placeholder (centered label); replaced
  in Step 7.
- `main.go` updated to use `NewMainWindow`.
- `go build ./...` and `go vet ./...` both pass.

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

## Step 10 — Scan Progress Reporting (future)

`Scan` already accepts `ctx context.Context` and a `progress func(float64)` callback
(nil = no-op). When this step is implemented, fill in the callback path:

1. Add `--progress` to the scanimage arguments when `progress != nil`.
2. Instead of buffering stderr, attach a pipe and read it line by line in a goroutine.
   - scanimage writes lines like `Progress: 42%` to stderr when `--progress` is given.
   - Parse the percentage and invoke `progress(pct / 100.0)` on each such line.
   - Collect any non-progress stderr text; treat it as an error at the end (existing
     behaviour unchanged for callers passing `nil`).
3. `cmd.Cancel` (set via `exec.CommandContext`) already handles cancellation via the
   passed `ctx`; no additional wiring needed.
4. In `ui/mainwindow.go`, pass a `progress` func that posts to a Fyne progress bar
   (or updates the spinner label) via `fyne.Do(...)`.
5. Add a unit test that feeds fake `Progress: N%` lines through `parseProgress` (new
   unexported helper) and asserts the correct float values.

---

## Step 11 — Build Verification

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
| 4 ✅ | `detect/edges.go` | OpenCV quad detection |
| 5 ✅ | `export/jpeg.go` | Axis-aligned + perspective JPEG export |
| 6 ✅ | `ui/mainwindow.go` | State machine, toolbar, button wiring |
| 7 | `ui/cropoverlay.go` | Custom widget (image, handles, loupe) |
| 8 | `ui/mainwindow.go` | Save handler wired to export + config |
| 9 | All | Polish, error paths, thread-safety audit |
| 10 | `scanner/scanner.go`, `ui/mainwindow.go` | Scan progress reporting |
| 11 | — | Build + smoke test |
