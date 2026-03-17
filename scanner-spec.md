# Quire — Document Scanner App Spec

## Overview

A desktop document scanning application for Arch Linux. The user clicks **Scan**, the app
acquires an image from the system scanner via `scanimage`, auto-detects the document edges
using OpenCV, and displays the image with a draggable crop box overlaid. The user can
accept the auto-detected crop or adjust it by dragging the corner/edge handles. While
dragging a handle, a loupe (zoom window) shows a magnified view of the image at that
handle's location. Once satisfied, they export the cropped result as a JPEG.

---

## Tech Stack

| Concern | Choice | Notes |
|---|---|---|
| Language | Go | Target Go 1.22+ |
| UI | Fyne v2 | `fyne.io/fyne/v2` |
| Image / CV | GoCV | `gocv.io/x/gocv` (OpenCV Go bindings) |
| Scanner | `scanimage` (CLI) | SANE's standard CLI tool, assumed installed |
| Output format | JPEG | via Go stdlib `image/jpeg` |

**Dependencies the agent must not substitute:**
- Do not replace GoCV with a pure-Go image library — the Canny+contour pipeline requires OpenCV.
- Do not use Fyne v1.
- Do not use native Go SANE bindings (e.g. `github.com/tjgq/sane`). That library is
  unmaintained and has incomplete device option support. `scanimage` subprocess is the
  correct interface to SANE.

---

## System Prerequisites (document for user, not for the agent to install)

```
sudo pacman -S sane opencv
yay -S gocv  # or vendor it via go.mod
```

The app assumes `scanimage` is on `$PATH` and that the user's scanner is already configured
and detected by SANE (`scanimage -L` lists it).

---

## Project Structure

```
quire/
├── go.mod
├── go.sum
├── main.go               # App entry point, Fyne window setup
├── scanner/
│   └── scanner.go        # scanimage subprocess logic
├── detect/
│   └── edges.go          # OpenCV edge detection + quad extraction
├── ui/
│   ├── cropoverlay.go    # Custom Fyne widget: image + draggable crop handles + loupe
│   └── mainwindow.go     # Top-level window layout and state
└── export/
    └── jpeg.go           # Crop + JPEG save
```

---

## Functional Requirements

### F1 — Scanner Discovery

- On startup, run `scanimage -L` and parse the output to get available device strings.
- If exactly one device is found, select it automatically.
- If multiple devices are found, show a dropdown in the toolbar to select one.
- If zero devices are found, show an error dialog and disable the Scan button.

### F2 — Scan Acquisition

- The **Scan** button triggers a subprocess call:
  ```
  scanimage --device <device> --format=pnm --mode Color --resolution 300
  ```
- Capture stdout as raw PNM bytes. On completion, decode into a Go `image.Image`.
- **Progressive display is not required.** Show a spinner/progress indicator while
  scanning is in progress. Keep the UI responsive (run subprocess in a goroutine).
- On subprocess error (non-zero exit or stderr output), show an error dialog with the
  stderr text.
- Resolution (75/150/300/600 dpi) should be selectable via a dropdown in the toolbar,
  defaulting to 300.
- Color mode (Color / Gray / Lineart) should be selectable via a dropdown, defaulting
  to Color.

### F3 — Edge Detection

Immediately after the scan image is decoded, run the auto-detect pipeline:

1. Convert to grayscale.
2. Apply Gaussian blur (kernel 5×5).
3. Run Canny edge detection (low threshold 75, high threshold 200 — make these tunable
   constants at the top of `edges.go`).
4. Find external contours.
5. For each contour, compute its convex hull, then approximate it with
   `ApproxPolyDP` (epsilon = 0.02 × arc length).
6. Among all approximated polygons with exactly 4 vertices, pick the one with the
   largest area.
7. If no valid quad is found, fall back to the full image bounding rect.
8. Return the 4 corner points as `image.Point` values, ordered: top-left, top-right,
   bottom-right, bottom-left.

Store the result of this detection as `detectedQuad` on the window state struct. It is
computed once per scan and never recomputed.

### F4 — Crop Overlay Widget

This is the core interactive element. It must be a custom Fyne widget (`widget.BaseWidget`).

#### 4a — Layout

- The scanned image fills the widget area, scaled to fit while preserving aspect ratio
  (letterboxed, not stretched).
- The crop box is drawn as a semi-transparent rectangle overlay (white border, ~2px,
  with a dark semi-transparent fill outside the crop region to dim the discarded area).

#### 4b — Crop Box Modes

The crop box has two modes, toggled by a checkbox/toggle labelled **"Free quad"** in the
bottom toolbar:

- **Axis-aligned mode** (default): The crop region is always a rectangle aligned to the
  image axes. It is defined by two points (top-left corner + bottom-right corner). There
  are 8 handles: 4 corners + 4 edge midpoints, behaving as in a standard image crop UI.
  The auto-detected quad is converted to its axis-aligned bounding rect when this mode
  is active.
- **Free quad mode**: The crop region is an arbitrary quadrilateral with 4 independent
  corner handles (no edge-midpoint handles in this mode). Corner handles can be dragged
  freely. This enables perspective correction for photos of documents taken at an angle.

When switching from free quad → axis-aligned, snap the quad to its axis-aligned bounding
rect. When switching from axis-aligned → free quad, convert the rect to a quad with
corners at the rect's four corners (no visible change, but the representation changes).

#### 4c — Handles

- **Axis-aligned mode:** 8 handles (4 corners + 4 edge midpoints), 12×12 px squares,
  filled white with a dark border. Hit target: 24×24 px centered on each handle.
  - Corner handles: drag freely.
  - Edge midpoint handles: drag constrains to the perpendicular axis.
- **Free quad mode:** 4 corner handles only, same sizing.
- The crop box must not be draggable to a degenerate state (min width/height: 20px in
  display coordinates).
- All crop box coordinates must be stored in **image coordinates** (not display/widget
  coordinates). The widget converts between them using the current scale + offset from
  the letterbox calculation.

#### 4d — Loupe (Magnifier Window)

While the user is actively dragging any handle, display a loupe: a magnified view of the
region of the full-resolution scanned image immediately around the handle being dragged.

- **Loupe size:** 200×200 px on screen.
- **Magnification:** Show a 40×40 px region from the full-resolution image (i.e. ~5×
  zoom). Make the source region size a tunable constant (`LoupeSourceSize = 40`).
- **Loupe position:** Display in the corner of the CropOverlay widget that is farthest
  from the handle being dragged, so it does not obscure the handle.
- **Content:** The loupe shows the raw scan pixels at that location with a crosshair
  drawn in the center indicating the exact handle position.
- **Visibility:** The loupe appears on `MouseDown` on a handle and disappears on
  `MouseUp`. It is never shown when not dragging.
- **Implementation:** Draw the loupe as an overlay composited directly within the
  `CropOverlay` widget's own draw/`Refresh()` pass using `canvas.Raster` or equivalent.
  Do not open a separate OS window.

### F5 — Reset Crop

- A **Reset Crop** button restores the crop box to `detectedQuad` (the auto-detected
  result stored at scan time). It does **not** re-run edge detection.
- If the current mode is axis-aligned, restore to the axis-aligned bounding rect of
  `detectedQuad`. If free quad mode is active, restore to `detectedQuad` directly.

### F6 — Export

- A **Save** button opens a Fyne file save dialog.
- **Default directory:** The dialog should open in the directory last successfully saved
  to. Persist this across sessions by storing it in a small config file at
  `$XDG_CONFIG_HOME/quire/config.json` (fall back to `~/.config/quire/config.json`).
  On first launch (no config file), default to `~/Documents/`.
- **Default filename:** `scan.jpg`.
- The export pipeline:
  1. Extract the crop region from the original full-resolution scanned image.
  2. **Axis-aligned mode:** Simple sub-image crop (no transform needed).
  3. **Free quad mode:** Apply a perspective transform
     (`gocv.GetPerspectiveTransform` + `gocv.WarpPerspective`) to produce a flat,
     rectangular output. Output size is determined from the bounding rect of the four
     crop points (width = max(top-right.X − top-left.X, bottom-right.X − bottom-left.X),
     same for height).
  4. Encode as JPEG, quality 92.
  5. Write to the user-chosen path.
  6. On success: update the persisted last-save directory to the directory of the saved
     file, then show a success notification.
- Show a success snackbar/notification on completion, or an error dialog on failure.
- The **Save** button should be disabled until a scan has been acquired.

### F7 — State Machine

The app has four states; UI elements enable/disable accordingly:

| State | Scan btn | Save btn | Reset btn | Free quad toggle |
|---|---|---|---|---|
| `Idle` (no scan yet) | enabled | disabled | disabled | disabled |
| `Scanning` | disabled | disabled | disabled | disabled |
| `Ready` (scan acquired) | enabled | enabled | enabled | enabled |
| `Saving` | disabled | disabled | disabled | disabled |

---

## UI Layout

```
┌─────────────────────────────────────────────────┐
│  [Device ▼]  [Resolution ▼]  [Mode ▼]           │  ← toolbar
├─────────────────────────────────────────────────┤
│                                                 │
│           CropOverlay widget                    │
│           (fills remaining space)               │
│                                                 │
├─────────────────────────────────────────────────┤
│  [Reset Crop]  [□ Free quad]       [Scan] [Save]│  ← bottom bar
└─────────────────────────────────────────────────┘
```

- Minimum window size: 800×600.
- The CropOverlay widget expands to fill available space when the window is resized.
- Before any scan is acquired, the CropOverlay area shows placeholder text: "Press Scan
  to begin."
- **Scan** and **Save** are in the bottom-right corner of the bottom bar.
- **Reset Crop** and **Free quad** toggle are in the bottom-left corner of the bottom bar.

---

## Persistence

Config file: `$XDG_CONFIG_HOME/quire/config.json` (fallback: `~/.config/quire/config.json`)

```json
{
  "last_save_dir": "/home/user/Documents"
}
```

Read on startup, written after each successful save.

---

## Error Handling

- `scanimage` not on PATH → error dialog on startup, Scan button disabled.
- Scanner disconnected mid-scan → error dialog with stderr.
- OpenCV contour detection produces no valid quad → silently fall back to full image rect
  (do not show an error; auto-detect simply failed gracefully).
- File write failure on save → error dialog with OS error message.
- Config file unreadable/malformed → silently ignore, use defaults (do not crash).

---

## Non-Requirements (explicitly out of scope)

- Multi-page / ADF scanning.
- PDF export.
- Rotation/deskewing (beyond what perspective warp in free quad mode provides).
- Batch mode.
- Any network/cloud functionality.
- Windows or macOS support (Arch Linux only; do not add build tags or abstractions for
  other OSes).
- Progressive/streaming scan display.

---

## Notes for the Coding Agent

- Use `fyne.io/fyne/v2 v2.5.x` (latest stable as of early 2026).
- Use `gocv.io/x/gocv v0.37.x` or later.
- All goroutines that update Fyne widgets must do so via `fyne.Do()` or the Fyne
  thread-safety mechanism. The scan subprocess runs in a goroutine; all resulting UI
  updates must be posted back to the main thread.
- The `CropOverlay` widget's mouse handlers should be implemented via
  `desktop.Mouseable` and `desktop.Hoverable` interfaces, not via `widget.Tappable`
  alone.
- Do not use global variables for app state; pass state through struct fields on the
  main window struct.
- GoCV `Mat` objects must be explicitly `.Close()`'d — use `defer mat.Close()` at every
  allocation site to avoid memory leaks.
- The loupe overlay must be composited within the `CropOverlay` widget's own draw pass,
  not as a separate OS-level window.
- Axis-aligned mode and free quad mode share the same underlying storage type internally
  (always store 4 corner points); axis-aligned mode simply constrains them to a rect on
  every drag update.
