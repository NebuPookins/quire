package main

import (
	"fyne.io/fyne/v2/app"

	"quire/ui"
)

func main() {
	a := app.New()
	mw := ui.NewMainWindow(a)
	mw.Show()
}
