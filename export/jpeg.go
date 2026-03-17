package export

import (
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"os"
	"path/filepath"
)

const jpegQuality = 92

// SaveAxisAligned crops img to the rectangle defined by topLeft/bottomRight
// and writes the result as a JPEG (quality 92) to path atomically.
func SaveAxisAligned(img image.Image, topLeft, bottomRight image.Point, path string) error {
	r := image.Rectangle{Min: topLeft, Max: bottomRight}
	return writeJPEG(cropImage(img, r), path)
}

// SavePerspective applies a perspective warp to img using the four crop corners
// (ordered TL, TR, BR, BL) and writes the result as a JPEG (quality 92) to
// path atomically. The output dimensions are derived from the bounding rect of
// the four points per the spec:
//
//	width  = max(TR.X − TL.X, BR.X − BL.X)
//	height = max(BL.Y − TL.Y, BR.Y − TR.Y)
func SavePerspective(img image.Image, quad [4]image.Point, path string) error {
	tl, tr, br, bl := quad[0], quad[1], quad[2], quad[3]
	w := max(tr.X-tl.X, br.X-bl.X)
	h := max(bl.Y-tl.Y, br.Y-tr.Y)
	if w <= 0 || h <= 0 {
		return fmt.Errorf("degenerate quad: computed output size %dx%d", w, h)
	}
	warped, err := perspectiveWarp(img, quad, w, h)
	if err != nil {
		return fmt.Errorf("warp perspective: %w", err)
	}
	return writeJPEG(warped, path)
}

// cropImage returns a sub-image of img cropped to r.
// Uses SubImage when available; otherwise copies pixels.
func cropImage(img image.Image, r image.Rectangle) image.Image {
	type subImager interface {
		SubImage(image.Rectangle) image.Image
	}
	if si, ok := img.(subImager); ok {
		return si.SubImage(r)
	}
	dst := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(dst, dst.Bounds(), img, r.Min, draw.Src)
	return dst
}

// writeJPEG encodes img as JPEG quality 92 and writes atomically to path.
func writeJPEG(img image.Image, path string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".quire-export-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if err := jpeg.Encode(tmp, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
