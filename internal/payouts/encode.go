package payouts

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"

	"github.com/rwcarlsen/goexif/exif"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	ContentType = "image/jpeg"
	MaxBytes    = 20 << 20
	maxEdge     = 1600
	jpegQuality = 85
)

// EncodeJPEG reads an uploaded screenshot and returns a JPEG sized so
// payout text stays readable. Place and GPS metadata are ignored.
func EncodeJPEG(r io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("payouts: read: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("payouts: empty file")
	}
	if len(raw) > MaxBytes {
		return nil, fmt.Errorf("payouts: file too large")
	}

	orient := 1
	if x, err := exif.Decode(bytes.NewReader(raw)); err == nil {
		if tag, err := x.Get(exif.Orientation); err == nil {
			if n, err := tag.Int(0); err == nil {
				orient = n
			}
		}
	}

	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("payouts: unsupported image")
	}
	img = applyOrientation(img, orient)
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 1 || h < 1 {
		return nil, fmt.Errorf("payouts: invalid dimensions")
	}
	nw, nh := fitEdge(w, h, maxEdge)
	if nw != w || nh != h {
		dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
		img = dst
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, fmt.Errorf("payouts: jpeg: %w", err)
	}
	return buf.Bytes(), nil
}

func fitEdge(w, h, edge int) (int, int) {
	if w <= edge && h <= edge {
		return w, h
	}
	scale := float64(edge) / float64(w)
	if h > w {
		scale = float64(edge) / float64(h)
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

func applyOrientation(img image.Image, tag int) image.Image {
	switch tag {
	case 2:
		return flipH(img)
	case 3:
		return rotate180(img)
	case 4:
		return flipV(img)
	case 6:
		return rotate90CW(img)
	case 8:
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
