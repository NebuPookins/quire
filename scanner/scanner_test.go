package scanner

import (
	"bytes"
	"fmt"
	"image"
	"testing"
)

// --- parseDevices ---

func TestParseDevices_Single(t *testing.T) {
	out := "pixma:04A91749_284142|CANON PIXMA MG3600 multi-function peripheral\n"
	got := parseDevices(out)
	if len(got) != 1 {
		t.Fatalf("want 1 device, got %d: %v", len(got), got)
	}
	if got[0].Name != "pixma:04A91749_284142" {
		t.Errorf("unexpected Name: %q", got[0].Name)
	}
	if got[0].Description != "CANON PIXMA MG3600 multi-function peripheral" {
		t.Errorf("unexpected Description: %q", got[0].Description)
	}
}

func TestParseDevices_Multiple(t *testing.T) {
	out := "v4l:/dev/video0|Noname C270 HD WEBCAM virtual device\n" +
		"genesys:libusb:003:010|Canon LiDE 120 flatbed scanner\n"
	got := parseDevices(out)
	if len(got) != 2 {
		t.Fatalf("want 2 devices, got %d: %v", len(got), got)
	}
	if got[1].Name != "genesys:libusb:003:010" {
		t.Errorf("unexpected Name: %q", got[1].Name)
	}
	if got[1].Description != "Canon LiDE 120 flatbed scanner" {
		t.Errorf("unexpected Description: %q", got[1].Description)
	}
}

func TestParseDevices_Empty(t *testing.T) {
	if got := parseDevices(""); len(got) != 0 {
		t.Fatalf("expected no devices, got %v", got)
	}
}

func TestParseDevices_BlankLines(t *testing.T) {
	out := "genesys:libusb:003:010|Canon LiDE 120 flatbed scanner\n\n"
	got := parseDevices(out)
	if len(got) != 1 {
		t.Fatalf("want 1 device, got %d", len(got))
	}
}

func TestParseDevices_NoSeparator(t *testing.T) {
	// Lines without a | separator are silently skipped.
	if got := parseDevices("some unexpected line\n"); len(got) != 0 {
		t.Fatalf("expected no devices, got %v", got)
	}
}

// --- parseDeviceOptions ---

const sampleAllOptions = `
All options specific to device 'genesys:libusb:003:010':
  Scan Mode:
    --mode Color|Gray [Gray]
        Selects the scan mode (e.g., lineart, monochrome, or color).
    --resolution 4800|2400|1200|600|300|150|100|75dpi [75]
        Sets the resolution of the scanned image.
  Geometry:
    -l 0..216mm [0]
`

func TestParseDeviceOptions_ModesAndResolutions(t *testing.T) {
	opts := parseDeviceOptions(sampleAllOptions)

	if len(opts.Modes) != 2 || opts.Modes[0] != "Color" || opts.Modes[1] != "Gray" {
		t.Errorf("unexpected modes: %v", opts.Modes)
	}
	if opts.DefaultMode != "Gray" {
		t.Errorf("unexpected DefaultMode: %q", opts.DefaultMode)
	}
	want := []int{4800, 2400, 1200, 600, 300, 150, 100, 75}
	if len(opts.Resolutions) != len(want) {
		t.Fatalf("want %d resolutions, got %d: %v", len(want), len(opts.Resolutions), opts.Resolutions)
	}
	for i, r := range opts.Resolutions {
		if r != want[i] {
			t.Errorf("resolution[%d]: got %d, want %d", i, r, want[i])
		}
	}
}

func TestParseDeviceOptions_LinepartMode(t *testing.T) {
	opts := parseDeviceOptions("    --mode Lineart|Gray|Color [Color]\n")
	if len(opts.Modes) != 3 || opts.Modes[0] != "Lineart" {
		t.Errorf("unexpected modes: %v", opts.Modes)
	}
}

func TestParseDeviceOptions_Empty(t *testing.T) {
	opts := parseDeviceOptions("")
	if len(opts.Modes) != 0 || len(opts.Resolutions) != 0 {
		t.Errorf("expected empty options, got %+v", opts)
	}
}

// --- decodePNM ---

func buildPPM(width, height int, fill [3]byte) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "P6\n%d %d\n255\n", width, height)
	row := make([]byte, width*3)
	for i := 0; i < width; i++ {
		row[i*3], row[i*3+1], row[i*3+2] = fill[0], fill[1], fill[2]
	}
	for range height {
		b.Write(row)
	}
	return b.Bytes()
}

func buildPGM(width, height int, fill byte) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "P5\n%d %d\n255\n", width, height)
	row := bytes.Repeat([]byte{fill}, width)
	for range height {
		b.Write(row)
	}
	return b.Bytes()
}

func buildPBM(width, height int, black bool) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "P4\n%d %d\n", width, height)
	rowBytes := (width + 7) / 8
	var fillByte byte
	if black {
		fillByte = 0xff
	}
	row := bytes.Repeat([]byte{fillByte}, rowBytes)
	for range height {
		b.Write(row)
	}
	return b.Bytes()
}

func TestDecodePNM_PPM(t *testing.T) {
	data := buildPPM(4, 3, [3]byte{255, 0, 128})
	img, err := decodePNM(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds() != (image.Rect(0, 0, 4, 3)) {
		t.Fatalf("unexpected bounds: %v", img.Bounds())
	}
	r, _, _, _ := img.At(0, 0).RGBA()
	if r>>8 != 255 {
		t.Fatalf("expected red=255, got %d", r>>8)
	}
}

func TestDecodePNM_PGM(t *testing.T) {
	data := buildPGM(8, 8, 128)
	img, err := decodePNM(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 8 || img.Bounds().Dy() != 8 {
		t.Fatalf("unexpected bounds: %v", img.Bounds())
	}
}

func TestDecodePNM_PBM_Black(t *testing.T) {
	data := buildPBM(8, 4, true)
	img, err := decodePNM(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := img.At(0, 0).RGBA()
	if r != 0 || g != 0 || b != 0 {
		t.Fatalf("expected black pixel, got r=%d g=%d b=%d", r, g, b)
	}
}

func TestDecodePNM_PBM_White(t *testing.T) {
	data := buildPBM(8, 4, false)
	img, err := decodePNM(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	r, _, _, _ := img.At(0, 0).RGBA()
	if r>>8 != 255 {
		t.Fatalf("expected white pixel, got r=%d", r>>8)
	}
}

func TestDecodePNM_UnknownMagic(t *testing.T) {
	_, err := decodePNM(bytes.NewReader([]byte("P3\n1 1\n255\n255 0 0\n")))
	if err == nil {
		t.Fatal("expected error for unsupported P3 format")
	}
}

func TestDecodePNM_PPMWithComment(t *testing.T) {
	data := []byte("P6\n# created by scanimage\n2 2\n255\n\xff\x00\x00\xff\x00\x00\xff\x00\x00\xff\x00\x00")
	img, err := decodePNM(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 2 {
		t.Fatalf("unexpected bounds: %v", img.Bounds())
	}
}
