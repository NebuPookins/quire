package detect

// Request that the linker only link libraries that are actually referenced,
// preventing opencv_viz (and its VTK dependency) from being pulled in.

// #cgo LDFLAGS: -Wl,--as-needed
import "C"
