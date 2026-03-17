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

## Step 7 — CropOverlay Widget (`ui/cropoverlay.go`) ✅ DONE

Files: `ui/cropoverlay.go`

- `CropOverlay` embeds `widget.BaseWidget`; compile-time interface checks for
  `desktop.Mouseable` and `desktop.Hoverable`.
- `CreateRenderer` returns a `cropRenderer` holding a `canvas.Raster` (pixel generator)
  and a `canvas.Text` placeholder shown only when no image is loaded.
- `letterbox(dispW, dispH, img)` — scale + offset for aspect-ratio-preserving fit.
- `imgToDisp` / `dispToImg` — convert between image and display coordinate spaces.
- `generate(w, h int) image.Image` — full pixel-level renderer:
  - Nearest-neighbour scaled image into the letterbox rect.
  - `applyDimOverlay`: axis-aligned fast path (4 dim rects); free-quad path uses
    `pointInQuad` (cross-product convex test, clockwise winding) per pixel within
    the image area only.
  - White 2px crop border via Bresenham `drawThickLine`.
  - 12×12 white handle squares with dark border (8 handles for axis-aligned,
    4 for free quad).
  - Loupe: 200×200 nearest-neighbour zoom of a 40×40 source region centred on the
    active handle; placed in the farthest corner; white crosshair drawn at centre.
- `handlePositions(scale, offX, offY)` — returns 8 or 4 fyne.Position values
  (corners + edge midpoints for axis-aligned; corners only for free quad).
- `hitTestHandle(pos)` — 24×24 pt hit target per handle; uses `widgetSize` (points).
- `applyDrag(pos)` — updates `cropPts` for the active handle; axis-aligned mode
  re-derives all four corners from updated min/max; enforces 20 display-unit minimum
  crop size; free-quad mode moves the corner directly.
- `MouseDown/Up/In/Out/Moved` wired to hit-test and drag.
- `go build ./...` and `go vet ./...` both pass.

---

## Step 8 — Save Handler (complete `ui/mainwindow.go`) ✅ DONE

Files: `ui/mainwindow.go`

- `onSave`: `setState(StateSaving)`, creates `dialog.NewFileSave` with callback.
  - Sets default filename `scan.jpg` and initial location from `cfg.LastSaveDir`
    (via `storage.NewFileURI` + `storage.ListerForURI`; silently skipped if unset
    or invalid).
  - On cancel (writer == nil): `setState(StateReady)`.
  - On confirmation: closes writer immediately (path extracted via `writer.URI().Path()`),
    launches goroutine calling `export.SavePerspective` (free-quad mode) or
    `export.SaveAxisAligned` (axis-aligned, using `cropPts[0]` and `cropPts[2]`).
  - `fyne.Do` on success: updates `cfg.LastSaveDir` (`filepath.Dir(path)`), calls
    `config.Save`, sends OS notification via `app.SendNotification`, `setState(StateReady)`.
  - `fyne.Do` on error: error dialog, `setState(StateReady)`.
- Imports added: `path/filepath`, `fyne.io/fyne/v2/storage`, `quire/export`.
- `go build ./...` and `go vet ./...` both pass.

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
- **CropOverlay rendering performance:** the current `canvas.Raster` generator scales
  the image and applies the dim overlay with per-pixel CPU loops, which is too slow for
  smooth dragging at HiDPI resolutions. Replace with:
  - A `canvas.Image` for the scanned image — Fyne uploads it as a GPU texture and
    handles scaling via OpenGL/Metal, so the expensive scale is free at drag time.
    Set `FillMode = canvas.ImageFillContain` for letterboxing.
  - A thin `canvas.Raster` drawn on top for the overlay only (dim regions, crop
    border, handles, loupe). This is much cheaper: no image scaling, just drawing
    on a widget-sized buffer.
  - For the dim regions in this overlay, use `image/draw` with
    `image.NewUniform(color.NRGBA{0, 0, 0, 165})` and `draw.Over` instead of
    per-pixel float multiplication.
  - For the free-quad dim path, `golang.org/x/image/draw` (already available
    transitively via Fyne) provides optimized scanline fill.

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
| 7 ✅ | `ui/cropoverlay.go` | Custom widget (image, handles, loupe) |
| 8 ✅ | `ui/mainwindow.go` | Save handler wired to export + config |
| 9 | All | Polish, error paths, thread-safety audit |
| 10 | `scanner/scanner.go`, `ui/mainwindow.go` | Scan progress reporting |
| 11 | — | Build + smoke test |
