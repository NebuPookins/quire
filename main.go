package main

import (
	"fyne.io/fyne/v2/app"

	"quire/ui"
)

func main() {
	a := app.NewWithID("net.nebupookins.quire")
	mw := ui.NewMainWindow(a)
	mw.Show()
}
