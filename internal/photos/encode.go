// Package photos encodes uploaded images for the dashboard photo frame.
//
// Images are stored as display-sized JPEG (quality 82). WebP encode needs CGO/
// libwebp, but the Docker build uses CGO_ENABLED=0, so JPEG is the portable choice.
package photos

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"strings"

	"github.com/rwcarlsen/goexif/exif"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	Ext         = ".jpg"
	ContentType = "image/jpeg"
	MaxBytes    = 20 << 20 // 20 MiB upload
	jpegQuality = 82
)

// Landscape / portrait max bounds for the 1280×800 stage.
const (
	maxLandW = 1280
	maxLandH = 800
	maxPortW = 800
	maxPortH = 1200
)

// Result is the encoded image plus EXIF-derived fields.
type Result struct {
	JPEG   []byte
	Orient string // "l" or "p"
	Date   string // YYYY-MM-DD
	Place  string // GPS reverse-geocode, else GPT from tags/filename
	Width  int
	Height int
}

// Encode reads an uploaded image, applies EXIF orientation, resizes to frame
// bounds, and returns a JPEG suitable for the kiosk display.
// filename is optional; used with tags to synthesize a place when GPS is absent.
func Encode(r io.Reader, filename string) (Result, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return Result{}, fmt.Errorf("photos: read: %w", err)
	}
	if len(raw) == 0 {
		return Result{}, fmt.Errorf("photos: empty file")
	}
	if len(raw) > MaxBytes {
		return Result{}, fmt.Errorf("photos: file too large (max %d bytes)", MaxBytes)
	}

	date := ""
	place := ""
	orientTag := 1
	var xif *exif.Exif
	if x, err := exif.Decode(bytes.NewReader(raw)); err == nil {
		xif = x
		if t, err := x.DateTime(); err == nil {
			date = t.Format("2006-01-02")
		}
		if tag, err := x.Get(exif.Orientation); err == nil {
			if n, err := tag.Int(0); err == nil {
				orientTag = n
			}
		}
		if lat, lon, err := x.LatLong(); err == nil {
			place = reversePlace(lat, lon)
		}
	}
	if place == "" {
		hints := collectHints(raw, xif, filename, date)
		place = synthesizePlace(hints)
	}

	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return Result{}, fmt.Errorf("photos: decode: %w", err)
	}
	img = applyOrientation(img, orientTag)

	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 1 || h < 1 {
		return Result{}, fmt.Errorf("photos: invalid dimensions")
	}

	maxW, maxH := maxLandW, maxLandH
	if h > w {
		maxW, maxH = maxPortW, maxPortH
	}
	nw, nh := fitWithin(w, h, maxW, maxH)
	if nw != w || nh != h {
		dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
		img = dst
		w, h = nw, nh
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return Result{}, fmt.Errorf("photos: jpeg: %w", err)
	}

	o := "l"
	if h > w {
		o = "p"
	}
	return Result{
		JPEG:   buf.Bytes(),
		Orient: o,
		Date:   date,
		Place:  place,
		Width:  w,
		Height: h,
	}, nil
}

func fitWithin(w, h, maxW, maxH int) (int, int) {
	if w <= maxW && h <= maxH {
		return w, h
	}
	rw := float64(maxW) / float64(w)
	rh := float64(maxH) / float64(h)
	scale := rw
	if rh < rw {
		scale = rh
	}
	nw := int(float64(w)*scale + 0.5)
	nh := int(float64(h)*scale + 0.5)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	return nw, nh
}

// applyOrientation rotates/flips per EXIF orientation tag (1–8).
func applyOrientation(img image.Image, tag int) image.Image {
	switch tag {
	case 2: // flip H
		return flipH(img)
	case 3: // 180
		return rotate180(img)
	case 4: // flip V
		return flipV(img)
	case 5: // transpose
		return rotate90CW(flipH(img))
	case 6: // 90 CW
		return rotate90CW(img)
	case 7: // transverse
		return rotate90CW(flipV(img))
	case 8: // 90 CCW
		return rotate90CCW(img)
	default:
		return img
	}
}

func flipH(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(b.Dx()-(x-b.Min.X)-1, y-b.Min.Y, src.At(x, y))
		}
	}
	return dst
}

func flipV(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(x-b.Min.X, b.Dy()-(y-b.Min.Y)-1, src.At(x, y))
		}
	}
	return dst
}

func rotate180(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(b.Dx()-(x-b.Min.X)-1, b.Dy()-(y-b.Min.Y)-1, src.At(x, y))
		}
	}
	return dst
}

func rotate90CW(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(h-1-y, x, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

func rotate90CCW(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(y, w-1-x, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// TagFromDate returns YYYY-MM used as the default pairing tag.
// Empty or short dates yield "" so undated photos are not forced into the current month.
func TagFromDate(date string) string {
	date = strings.TrimSpace(date)
	if len(date) >= 7 {
		return date[:7]
	}
	return ""
}
