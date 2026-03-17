# Quire

A desktop document scanning application for Arch Linux. Acquire an image from your
scanner, auto-detect document edges, adjust the crop box interactively, and export
a JPEG.

## Prerequisites

Install system dependencies:

```sh
sudo pacman -S sane opencv
```

Verify your scanner is recognised by SANE:

```sh
scanimage -L
```

Ensure `scanimage` is on your `$PATH`.

## Build

```sh
go build -o quire .
```

## Run

```sh
./quire
```

## Usage

1. On startup the app discovers available scanners. If multiple are found, select one
   from the **Device** dropdown in the toolbar.
2. Choose **Resolution** (75 / 150 / 300 / 600 dpi, default 300) and **Mode**
   (Color / Gray / Lineart, default Color).
3. Click **Scan**. A progress indicator is shown while scanning. The scanned image
   appears with an auto-detected crop box when done.
4. Adjust the crop box by dragging the handles. A loupe (magnified view) appears
   near each handle while dragging.
5. Toggle **Free quad** in the bottom bar to switch between an axis-aligned rectangle
   and a free four-corner quadrilateral (useful for perspective correction).
6. Click **Reset Crop** to restore the auto-detected crop.
7. Click **Save** to export the cropped region as a JPEG (quality 92). The file
   dialog opens in the last-used save directory, which is persisted across sessions.

## Configuration

A small config file is stored at:

```
$XDG_CONFIG_HOME/quire/config.json   # typically ~/.config/quire/config.json
```

It records the last directory used for saving. On first launch it defaults to
`~/Documents/`. The file is written automatically after each successful save; you
do not need to edit it manually.

## Tech Stack

| Concern | Library |
|---------|---------|
| UI | [Fyne v2](https://fyne.io) |
| Image / CV | [GoCV](https://gocv.io) (OpenCV bindings) |
| Scanner | `scanimage` (SANE CLI) |
| Output | JPEG via Go stdlib |
