package ui

// Regenerate bundled.go by running: go generate ./ui/
// (requires fyne in PATH and build/icons PNGs — run `make icons` first)

//go:generate fyne bundle --pkg ui --name AppIcon --output bundled.go ../build/icons/quire_256.png
//go:generate fyne bundle --pkg ui --name AppLogo --append --output bundled.go ../build/icons/quire_app_logo.png
