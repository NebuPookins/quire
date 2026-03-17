package detect

import (
	"image"
	"sort"

	"gocv.io/x/gocv"
)

const (
	CannyLow        = 75
	CannyHigh       = 200
	LoupeSourceSize = 40
)

// DetectQuad finds the largest 4-sided contour in img and returns its corners
// ordered: top-left, top-right, bottom-right, bottom-left.
// Falls back to the full image bounding rect if no quad is found.
func DetectQuad(img image.Image) [4]image.Point {
	b := img.Bounds()
	fallback := [4]image.Point{
		{X: b.Min.X, Y: b.Min.Y},
		{X: b.Max.X, Y: b.Min.Y},
		{X: b.Max.X, Y: b.Max.Y},
		{X: b.Min.X, Y: b.Max.Y},
	}

	mat, err := gocv.ImageToMatRGB(img)
	if err != nil {
		return fallback
	}
	defer mat.Close()

	gray := gocv.NewMat()
	defer gray.Close()
	gocv.CvtColor(mat, &gray, gocv.ColorRGBToGray)

	blurred := gocv.NewMat()
	defer blurred.Close()
	gocv.GaussianBlur(gray, &blurred, image.Point{X: 5, Y: 5}, 0, 0, gocv.BorderDefault)

	edges := gocv.NewMat()
	defer edges.Close()
	gocv.Canny(blurred, &edges, CannyLow, CannyHigh)

	contours := gocv.FindContours(edges, gocv.RetrievalExternal, gocv.ChainApproxSimple)
	defer contours.Close()

	var bestPts []image.Point
	bestArea := 0.0

	for i := 0; i < contours.Size(); i++ {
		pts, area := processContour(contours.At(i))
		if area > bestArea {
			bestArea = area
			bestPts = pts
		}
	}

	if bestPts == nil {
		return fallback
	}
	return orderQuad(bestPts)
}

// processContour computes the convex hull of a contour, approximates it with a
// polygon, and returns the 4 corner points and their enclosed area.
// Returns (nil, 0) if the approximation does not produce exactly 4 vertices.
func processContour(contour gocv.PointVector) ([]image.Point, float64) {
	// Compute convex hull as indices into the contour.
	hullIdx := gocv.NewMat()
	defer hullIdx.Close()
	if err := gocv.ConvexHull(contour, &hullIdx, false, false); err != nil {
		return nil, 0
	}

	// Rebuild hull as a PointVector using the returned indices.
	hull := gocv.NewPointVector()
	defer hull.Close()
	for j := 0; j < hullIdx.Rows(); j++ {
		hull.Append(contour.At(int(hullIdx.GetIntAt(j, 0))))
	}

	// Approximate the hull with a polygon (ε = 2% of arc length).
	epsilon := 0.02 * gocv.ArcLength(hull, true)
	approx := gocv.ApproxPolyDP(hull, epsilon, true)
	defer approx.Close()

	if approx.Size() != 4 {
		return nil, 0
	}

	pts := make([]image.Point, 4)
	for k := 0; k < 4; k++ {
		pts[k] = approx.At(k)
	}
	return pts, gocv.ContourArea(approx)
}

// orderQuad orders 4 points as: top-left, top-right, bottom-right, bottom-left.
func orderQuad(pts []image.Point) [4]image.Point {
	sort.Slice(pts, func(i, j int) bool { return pts[i].Y < pts[j].Y })
	top, bottom := pts[:2], pts[2:]
	sort.Slice(top, func(i, j int) bool { return top[i].X < top[j].X })
	sort.Slice(bottom, func(i, j int) bool { return bottom[i].X < bottom[j].X })
	return [4]image.Point{top[0], top[1], bottom[1], bottom[0]}
}
