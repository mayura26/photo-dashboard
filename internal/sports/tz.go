package sports

import (
	"os"
	"strings"
	"time"
)

const defaultDisplayTZ = "Australia/Sydney"

// DisplayLocation resolves the IANA zone for sports kickoff labels.
// Order: SPORTS_TZ, TZ, then Australia/Sydney. Falls back to Australia/Sydney
// if the chosen name cannot be loaded (e.g. missing tzdata).
func DisplayLocation() (*time.Location, string) {
	name := strings.TrimSpace(os.Getenv("SPORTS_TZ"))
	if name == "" {
		name = strings.TrimSpace(os.Getenv("TZ"))
	}
	if name == "" {
		name = defaultDisplayTZ
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		fallback, _ := time.LoadLocation(defaultDisplayTZ)
		if fallback == nil {
			fallback = time.UTC
		}
		return fallback, defaultDisplayTZ
	}
	return loc, name
}

func ensureLoc(loc *time.Location) *time.Location {
	if loc != nil {
		return loc
	}
	loc, _ = DisplayLocation()
	return loc
}
