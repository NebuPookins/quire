package ui

import (
	"image"
	"testing"

	"fyne.io/fyne/v2/test"

	"quire/scanner"
)

// TestScanCropPersistence is a regression test: the second scan must display
// the crop the user set after the first scan, not reset to the full image bounds.
func TestScanCropPersistence(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	mw := NewMainWindow(a)

	img := image.NewGray(image.Rect(0, 0, 100, 80))

	// First scan: expect full image bounds.
	mw.applyScanResult(img)
	wantFull := [4]image.Point{{0, 0}, {100, 0}, {100, 80}, {0, 80}}
	if got := mw.cropOverlay.CurrentCrop(); got != wantFull {
		t.Fatalf("first scan: crop = %v, want full bounds %v", got, wantFull)
	}

	// User adjusts the crop handles.
	userCrop := [4]image.Point{{10, 10}, {90, 10}, {90, 70}, {10, 70}}
	mw.cropOverlay.SetCrop(userCrop, false)

	// Second scan: applyScanResult must reuse the overlay's existing crop,
	// not overwrite it with the full image bounds.
	mw.applyScanResult(img)
	if got := mw.cropOverlay.CurrentCrop(); got != userCrop {
		t.Errorf("second scan: crop = %v, want user's crop %v", got, userCrop)
	}
}

// TestSetState verifies the F7 state table: which buttons are enabled/disabled
// in each AppState.
func TestSetState(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	mw := NewMainWindow(a)

	cases := []struct {
		name         string
		setup        func()
		state        AppState
		scanEnabled  bool
		saveEnabled  bool
		resetEnabled bool
	}{
		{
			name:         "BootingUp — all disabled",
			setup:        func() { mw.selectedDevice = scanner.Device{Name: "test:dev"} },
			state:        StateBootingUp,
			scanEnabled:  false,
			saveEnabled:  false,
			resetEnabled: false,
		},
		{
			name:         "Idle/no device — scan disabled",
			setup:        func() { mw.selectedDevice = scanner.Device{} },
			state:        StateIdle,
			scanEnabled:  false,
			saveEnabled:  false,
			resetEnabled: false,
		},
		{
			name:         "Idle/device selected — scan enabled",
			setup:        func() { mw.selectedDevice = scanner.Device{Name: "test:dev"} },
			state:        StateIdle,
			scanEnabled:  true,
			saveEnabled:  false,
			resetEnabled: false,
		},
		{
			name:         "Scanning — all disabled",
			setup:        func() {},
			state:        StateScanning,
			scanEnabled:  false,
			saveEnabled:  false,
			resetEnabled: false,
		},
		{
			name:         "Ready — scan/save/reset all enabled",
			setup:        func() {},
			state:        StateReady,
			scanEnabled:  true,
			saveEnabled:  true,
			resetEnabled: true,
		},
		{
			name:         "Saving — all disabled",
			setup:        func() {},
			state:        StateSaving,
			scanEnabled:  false,
			saveEnabled:  false,
			resetEnabled: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()
			mw.setState(tc.state)
			if got := !mw.scanBtn.Disabled(); got != tc.scanEnabled {
				t.Errorf("scanBtn: enabled=%v, want %v", got, tc.scanEnabled)
			}
			if got := !mw.saveBtn.Disabled(); got != tc.saveEnabled {
				t.Errorf("saveBtn: enabled=%v, want %v", got, tc.saveEnabled)
			}
			if got := !mw.resetBtn.Disabled(); got != tc.resetEnabled {
				t.Errorf("resetBtn: enabled=%v, want %v", got, tc.resetEnabled)
			}
		})
	}
}
