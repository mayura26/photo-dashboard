package photos

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	geoHTTP     = &http.Client{Timeout: 6 * time.Second}
	geoMu       sync.Mutex
	geoLastCall time.Time
)

// reversePlace looks up a short place label from GPS via OpenStreetMap Nominatim.
// Returns "" on any failure (upload must not depend on network geo).
func reversePlace(lat, lon float64) string {
	// Nominatim asks for ≤1 req/s.
	geoMu.Lock()
	wait := time.Second - time.Since(geoLastCall)
	if wait > 0 {
		time.Sleep(wait)
	}
	geoLastCall = time.Now()
	geoMu.Unlock()

	u := fmt.Sprintf(
		"https://nominatim.openstreetmap.org/reverse?format=jsonv2&lat=%f&lon=%f&zoom=14&addressdetails=1",
		lat, lon,
	)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "photo-dashboard/1.0 (family photo frame)")
	req.Header.Set("Accept", "application/json")

	res, err := geoHTTP.Do(req)
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return ""
	}

	var payload struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Address     struct {
			Neighbourhood string `json:"neighbourhood"`
			Suburb        string `json:"suburb"`
			Village       string `json:"village"`
			Town          string `json:"town"`
			City          string `json:"city"`
			Municipality  string `json:"municipality"`
			County        string `json:"county"`
			State         string `json:"state"`
			Country       string `json:"country"`
			CountryCode   string `json:"country_code"`
		} `json:"address"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return ""
	}

	a := payload.Address
	local := firstNonEmpty(a.Neighbourhood, a.Suburb, a.Village, a.Town, a.City, a.Municipality, payload.Name)
	region := firstNonEmpty(a.State, a.County)
	if local != "" && region != "" && !strings.EqualFold(local, region) {
		return local + ", " + region
	}
	if local != "" {
		if a.Country != "" && !strings.EqualFold(local, a.Country) {
			return local + ", " + a.Country
		}
		return local
	}
	if region != "" {
		return region
	}
	// Last resort: first two comma segments of display_name.
	parts := strings.Split(payload.DisplayName, ",")
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[0]) + ", " + strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(payload.DisplayName)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}
