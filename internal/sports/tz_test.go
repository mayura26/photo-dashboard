package sports

import (
	"testing"
)

func TestDisplayLocationDefault(t *testing.T) {
	t.Setenv("SPORTS_TZ", "")
	t.Setenv("TZ", "")
	loc, name := DisplayLocation()
	if name != defaultDisplayTZ {
		t.Fatalf("name = %q, want %q", name, defaultDisplayTZ)
	}
	if loc == nil || loc.String() != defaultDisplayTZ {
		t.Fatalf("loc = %v", loc)
	}
}

func TestDisplayLocationSportsTZ(t *testing.T) {
	t.Setenv("SPORTS_TZ", "Pacific/Auckland")
	t.Setenv("TZ", "Australia/Sydney")
	loc, name := DisplayLocation()
	if name != "Pacific/Auckland" {
		t.Fatalf("name = %q, want Pacific/Auckland", name)
	}
	if loc.String() != "Pacific/Auckland" {
		t.Fatalf("loc = %v", loc)
	}
}

func TestDisplayLocationInvalidFallsBack(t *testing.T) {
	t.Setenv("SPORTS_TZ", "Not/ARealZone")
	t.Setenv("TZ", "")
	loc, name := DisplayLocation()
	if name != defaultDisplayTZ {
		t.Fatalf("name = %q, want %q", name, defaultDisplayTZ)
	}
	if loc == nil {
		t.Fatal("loc nil")
	}
}
