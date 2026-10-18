package photos

import "testing"

func TestPlaceHintsUseful(t *testing.T) {
	if (PlaceHints{Filename: "IMG_1234.jpg"}).useful() {
		t.Fatal("generic camera name should not be useful")
	}
	if !(PlaceHints{Filename: "Tofino surf trip.jpg"}).useful() {
		t.Fatal("descriptive filename should be useful")
	}
	if !(PlaceHints{Keywords: []string{"Banff", "lake"}}).useful() {
		t.Fatal("keywords should be useful")
	}
	if !(PlaceHints{XMPCity: "Sydney"}).useful() {
		t.Fatal("xmp city should be useful")
	}
}

func TestSplitTags(t *testing.T) {
	got := splitTags("Banff; lake, hike/summer")
	if len(got) != 4 {
		t.Fatalf("got %#v", got)
	}
}
