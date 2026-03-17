package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"

	"quire/scanner"
)

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
