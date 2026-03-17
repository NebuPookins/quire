package scanner

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// ErrScanImageNotFound is returned when scanimage is not on PATH.
var ErrScanImageNotFound = errors.New("scanimage not found on PATH")

// Mode is a scan colour mode as reported by the device (e.g. "Color", "Gray", "Lineart").
// The supported modes for a given device are returned by QueryOptions.
type Mode string

// Device represents a SANE scanner device.
type Device struct {
	// Name is the device identifier passed to scanimage --device.
	Name string
	// Description is the human-readable label shown in the UI (e.g. "Canon LiDE 120 flatbed scanner").
	Description string
}

// parseDevices extracts Device values from the output of
// scanimage --formatted-device-list=%d|%v %m %t.
// Each non-empty line has the form: <device-id>|<vendor> <model> <type>
func parseDevices(output string) []Device {
	var devices []Device
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, desc, found := strings.Cut(line, "|")
		if !found {
			continue
		}
		devices = append(devices, Device{Name: name, Description: strings.TrimSpace(desc)})
	}
	return devices
}

// ListDevices returns the SANE devices reported by scanimage.
// Returns ErrScanImageNotFound if scanimage is not on PATH.
func ListDevices() ([]Device, error) {
	bin, err := exec.LookPath("scanimage")
	if err != nil {
		return nil, ErrScanImageNotFound
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bin, "--formatted-device-list=%d|%v %m %t\n")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("scanimage --formatted-device-list: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return parseDevices(stdout.String()), nil
}

// DeviceOptions holds the scan options supported by a specific device.
type DeviceOptions struct {
	Modes       []Mode
	DefaultMode Mode  // device-reported default, e.g. "Gray"; empty if not advertised
	Resolutions []int
}

// parseDeviceOptions extracts supported modes and resolutions from the stdout of
// scanimage --device-name <device> --all-options.
//
// Relevant line formats (after trimming leading whitespace):
//
//	--mode Color|Gray [Gray]
//	--resolution 4800|2400|1200|600|300|150|100|75dpi [75]
//
// The "dpi" suffix may appear on any value; values are listed highest-first.
// The value in square brackets is the device-reported default.
func parseDeviceOptions(output string) DeviceOptions {
	var opts DeviceOptions
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if vals, ok := strings.CutPrefix(line, "--mode "); ok {
			values, rest, _ := strings.Cut(vals, " ")
			for _, v := range strings.Split(values, "|") {
				if v != "" {
					opts.Modes = append(opts.Modes, Mode(v))
				}
			}
			// Extract default from "[Gray]" bracket.
			if def, ok := strings.CutPrefix(strings.TrimSpace(rest), "["); ok {
				def, _, _ = strings.Cut(def, "]")
				opts.DefaultMode = Mode(def)
			}
		}
		if vals, ok := strings.CutPrefix(line, "--resolution "); ok {
			vals, _, _ = strings.Cut(vals, " ")
			for _, v := range strings.Split(vals, "|") {
				v = strings.TrimSuffix(v, "dpi")
				if n, err := strconv.Atoi(v); err == nil {
					opts.Resolutions = append(opts.Resolutions, n)
				}
			}
		}
	}
	return opts
}

// QueryOptions returns the scan options supported by the given device by running
// scanimage --device-name <device> --all-options.
func QueryOptions(device string) (DeviceOptions, error) {
	bin, err := exec.LookPath("scanimage")
	if err != nil {
		return DeviceOptions{}, ErrScanImageNotFound
	}
	var stdout bytes.Buffer
	cmd := exec.Command(bin, "--device-name", device, "--all-options")
	cmd.Stdout = &stdout
	// stderr intentionally discarded: scanimage writes harmless format warnings there.
	if err := cmd.Run(); err != nil {
		return DeviceOptions{}, fmt.Errorf("scanimage --all-options: %w", err)
	}
	return parseDeviceOptions(stdout.String()), nil
}

// Scan acquires an image from the given device.
// On non-zero exit or any stderr output, an error is returned wrapping the stderr text.
//
// progress is an optional callback invoked with values in [0, 1] as scan data arrives.
// Pass nil if progress reporting is not needed.
// When non-nil, the future implementation will pass --progress to scanimage and parse
// its stderr output; the callback will be invoked on the caller's goroutine.
//
// mode and resolution are optional: pass nil to omit the corresponding flag and let
// the device use its own default. This is appropriate when QueryOptions returns no
// values for that option.
func Scan(ctx context.Context, device string, mode *Mode, resolution *int, progress func(float64)) (image.Image, error) {
	bin, err := exec.LookPath("scanimage")
	if err != nil {
		return nil, ErrScanImageNotFound
	}
	args := []string{"--device", device, "--format=pnm"}
	if mode != nil {
		args = append(args, "--mode", string(*mode))
	}
	if resolution != nil {
		args = append(args, "--resolution", strconv.Itoa(*resolution))
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if stderrText := strings.TrimSpace(stderr.String()); stderrText != "" {
		return nil, fmt.Errorf("scanimage: %s", stderrText)
	}
	if runErr != nil {
		return nil, fmt.Errorf("scanimage: %w", runErr)
	}
	img, err := decodePNM(bytes.NewReader(stdout.Bytes()))
	if err != nil {
		return nil, fmt.Errorf("decode PNM: %w", err)
	}
	return img, nil
}

// decodePNM decodes a binary PNM image. Supports P4 (PBM), P5 (PGM), P6 (PPM),
// which correspond to scanimage's Lineart, Gray, and Color modes respectively.
func decodePNM(r io.Reader) (image.Image, error) {
	br := bufio.NewReader(r)

	magic, err := readToken(br)
	if err != nil {
		return nil, fmt.Errorf("read magic: %w", err)
	}

	width, err := readInt(br)
	if err != nil {
		return nil, fmt.Errorf("read width: %w", err)
	}
	height, err := readInt(br)
	if err != nil {
		return nil, fmt.Errorf("read height: %w", err)
	}

	switch magic {
	case "P6": // binary PPM — Color
		maxval, err := readInt(br)
		if err != nil {
			return nil, fmt.Errorf("read maxval: %w", err)
		}
		return decodePPM(br, width, height, maxval)
	case "P5": // binary PGM — Gray
		maxval, err := readInt(br)
		if err != nil {
			return nil, fmt.Errorf("read maxval: %w", err)
		}
		return decodePGM(br, width, height, maxval)
	case "P4": // binary PBM — Lineart
		return decodePBM(br, width, height)
	default:
		return nil, fmt.Errorf("unsupported PNM magic %q", magic)
	}
}

func decodePPM(r io.Reader, width, height, maxval int) (image.Image, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	buf := make([]byte, width*3)
	scale := uint32(0xffff)
	if maxval != 65535 {
		scale = uint32(0xffff) / uint32(maxval)
	}
	for y := range height {
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		for x := range width {
			r8, g8, b8 := buf[x*3], buf[x*3+1], buf[x*3+2]
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(uint32(r8) * scale >> 8),
				G: uint8(uint32(g8) * scale >> 8),
				B: uint8(uint32(b8) * scale >> 8),
				A: 0xff,
			})
		}
	}
	return img, nil
}

func decodePGM(r io.Reader, width, height, maxval int) (image.Image, error) {
	img := image.NewGray(image.Rect(0, 0, width, height))
	buf := make([]byte, width)
	scale := uint32(0xff)
	if maxval != 255 {
		scale = uint32(0xff) / uint32(maxval)
	}
	for y := range height {
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		for x := range width {
			img.SetGray(x, y, color.Gray{Y: uint8(uint32(buf[x]) * scale)})
		}
	}
	return img, nil
}

func decodePBM(r io.Reader, width, height int) (image.Image, error) {
	img := image.NewGray(image.Rect(0, 0, width, height))
	rowBytes := (width + 7) / 8
	buf := make([]byte, rowBytes)
	for y := range height {
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		for x := range width {
			bit := (buf[x/8] >> (7 - uint(x%8))) & 1
			// In PBM, 1 = black, 0 = white
			if bit == 1 {
				img.SetGray(x, y, color.Gray{Y: 0})
			} else {
				img.SetGray(x, y, color.Gray{Y: 0xff})
			}
		}
	}
	return img, nil
}

// readToken reads the next whitespace-delimited token, skipping PNM comments.
func readToken(br *bufio.Reader) (string, error) {
	skipWhitespaceAndComments(br)
	var sb strings.Builder
	for {
		b, err := br.ReadByte()
		if err != nil {
			if sb.Len() > 0 {
				return sb.String(), nil
			}
			return "", err
		}
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			break
		}
		sb.WriteByte(b)
	}
	return sb.String(), nil
}

func readInt(br *bufio.Reader) (int, error) {
	tok, err := readToken(br)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(tok)
}

func skipWhitespaceAndComments(br *bufio.Reader) {
	for {
		b, err := br.ReadByte()
		if err != nil {
			return
		}
		if b == '#' {
			// skip rest of line
			for {
				c, err := br.ReadByte()
				if err != nil || c == '\n' {
					break
				}
			}
			continue
		}
		if b != ' ' && b != '\t' && b != '\n' && b != '\r' {
			br.UnreadByte()
			return
		}
	}
}
