package main

import (
	"fyne.io/fyne/v2/app"

	"quire/ui"
)

func main() {
	a := app.NewWithID("net.nebupookins.quire")
	a.SetIcon(ui.AppIcon)
	mw := ui.NewMainWindow(a)
	mw.Show()
}
