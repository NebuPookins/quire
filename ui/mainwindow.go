package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// AppState represents the top-level application state.
type AppState int

const (
	StateIdle     AppState = iota // no scan yet
	StateScanning                 // scan in progress
	StateReady                    // scan acquired, ready to edit/save
	StateSaving                   // save in progress
)

// MainWindow holds all application state and top-level UI references.
type MainWindow struct {
	window fyne.Window
	app    fyne.App
	state  AppState

	scanBtn     *widget.Button
	saveBtn     *widget.Button
	resetBtn    *widget.Button
	freeQuadChk *widget.Check
	deviceSel   *widget.Select
	resSel      *widget.Select
	modeSel     *widget.Select
	cropOverlay *CropOverlay
}

// NewMainWindow constructs and returns the main application window.
func NewMainWindow(a fyne.App) *MainWindow {
	panic("not implemented")
}

// setState enables/disables UI controls according to the F7 state table.
func (mw *MainWindow) setState(s AppState) {
	panic("not implemented")
}
