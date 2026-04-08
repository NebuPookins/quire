BINARY     := quire
FYNE       := $(shell command -v fyne 2>/dev/null || echo $(HOME)/go/bin/fyne)

ICON_SIZES := 16 32 48 64 128 256 512
ICON_PNGS  := $(foreach s,$(ICON_SIZES),build/icons/quire_$(s).png)
LOGO_PNG   := build/icons/quire_app_logo.png

GO_SOURCES := $(shell find . -name '*.go' -not -name 'bundled.go' -not -path './.git/*')

PREFIX     := /usr
DESTDIR    :=

.PHONY: all icons generate test clean install
.NOTPARALLEL:

## all: build the binary (default target)
all: $(BINARY)

## $(BINARY): compile the Go binary
$(BINARY): ui/bundled.go $(GO_SOURCES)
	go build -o $@ .

## ui/bundled.go: bundle PNG assets into Go source via fyne bundle
ui/bundled.go: build/icons/quire_256.png $(LOGO_PNG)
	$(FYNE) bundle --pkg ui --name AppIcon --output $@ build/icons/quire_256.png
	$(FYNE) bundle --pkg ui --name AppLogo --append --output $@ $(LOGO_PNG)

## Icon PNGs: render quire_icon.svg at each required size
$(ICON_PNGS): build/icons/quire_%.png: quire_icon.svg | build/icons
	inkscape --export-type=png \
		--export-width=$* \
		--export-height=$* \
		--export-filename=$@ \
		quire_icon.svg 2>/dev/null

## Logo PNG: render quire_app_logo.svg at 2x (560x140) for HiDPI
$(LOGO_PNG): quire_app_logo.svg | build/icons
	inkscape --export-type=png \
		--export-width=560 \
		--export-height=140 \
		--export-filename=$@ \
		quire_app_logo.svg 2>/dev/null

build/icons:
	mkdir -p $@

## icons: render all PNG assets from SVG sources
icons: $(ICON_PNGS) $(LOGO_PNG)

## generate: regenerate ui/bundled.go (alias for go generate ./...)
generate: ui/bundled.go

## test: run all tests
test:
	go test ./...

## install: install binary, icons, desktop file, and license into DESTDIR
install: $(BINARY) $(ICON_PNGS)
	install -Dm755 $(BINARY)        $(DESTDIR)$(PREFIX)/bin/$(BINARY)
	install -Dm644 quire.desktop    $(DESTDIR)$(PREFIX)/share/applications/quire.desktop
	install -Dm644 LICENSE          $(DESTDIR)$(PREFIX)/share/licenses/quire/LICENSE
	$(foreach s,$(ICON_SIZES), \
		install -Dm644 build/icons/quire_$(s).png \
			$(DESTDIR)$(PREFIX)/share/icons/hicolor/$(s)x$(s)/apps/quire.png;)

## clean: remove the binary and all generated artifacts
clean:
	rm -f $(BINARY)
	rm -rf build/
	rm -f ui/bundled.go
