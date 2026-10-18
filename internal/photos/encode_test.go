package photos

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestEncodeLandscape(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2000, 1200))
	for y := 0; y < 1200; y++ {
		for x := 0; x < 2000; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	res, err := Encode(&buf, "cottage-weekend.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if res.Orient != "l" {
		t.Fatalf("orient: %s", res.Orient)
	}
	if res.Width > maxLandW || res.Height > maxLandH {
		t.Fatalf("size %dx%d exceeds landscape max", res.Width, res.Height)
	}
	if len(res.JPEG) == 0 {
		t.Fatal("empty jpeg")
	}
	if res.Date != "" {
		t.Fatalf("expected empty date without EXIF, got %q", res.Date)
	}
}

func TestEncodePortrait(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 900, 1600))
	for y := 0; y < 1600; y++ {
		for x := 0; x < 900; x++ {
			img.Set(x, y, color.RGBA{R: 50, G: 100, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	res, err := Encode(&buf, "IMG_0001.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if res.Orient != "p" {
		t.Fatalf("orient: %s", res.Orient)
	}
	if res.Width > maxPortW || res.Height > maxPortH {
		t.Fatalf("size %dx%d exceeds portrait max", res.Width, res.Height)
	}
}

func TestTagFromDate(t *testing.T) {
	if got := TagFromDate("2024-05-19"); got != "2024-05" {
		t.Fatalf("got %s", got)
	}
	if got := TagFromDate(""); got != "" {
		t.Fatalf("empty date tag: %q", got)
	}
}

func TestValidID(t *testing.T) {
	id, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	if !ValidID(id) {
		t.Fatalf("new id invalid: %s", id)
	}
	if ValidID("../etc") || ValidID("") {
		t.Fatal("bad ids accepted")
	}
}
