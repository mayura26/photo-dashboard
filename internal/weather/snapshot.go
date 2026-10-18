package weather

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// RealRainMM is the shower amount above which we treat an hour as real rain.
const RealRainMM = 0.2

// Snapshot is the dashboard weather payload (matches DATA.weather).
type Snapshot struct {
	Hi       int       `json:"hi"`
	Lo       int       `json:"lo"`
	Feels    int       `json:"feels"`
	Humidity int       `json:"humidity"`
	Wind     int       `json:"wind"`
	T        []int     `json:"t"`
	C        []string  `json:"c"`
	P        []float64 `json:"p"` // hourly shower mm
}

// BuildSnapshot maps an Open-Meteo response into the dashboard weather shape.
func BuildSnapshot(om openMeteoResponse) (json.RawMessage, error) {
	n := len(om.Hourly.Time)
	if n < 24 ||
		len(om.Hourly.ApparentTemperature) < 24 ||
		len(om.Hourly.WeatherCode) < 24 ||
		len(om.Hourly.Showers) < 24 {
		return nil, fmt.Errorf("open-meteo: need 24 hourly points, got %d", n)
	}
	if len(om.Daily.ApparentTemperatureMax) < 1 || len(om.Daily.ApparentTemperatureMin) < 1 {
		return nil, fmt.Errorf("open-meteo: missing daily high/low")
	}

	sunrise, sunset := parseDayBounds(om)

	t := make([]int, 24)
	c := make([]string, 24)
	p := make([]float64, 24)
	for i := 0; i < 24; i++ {
		t[i] = int(math.Round(om.Hourly.ApparentTemperature[i]))
		p[i] = math.Round(om.Hourly.Showers[i]*10) / 10
		hour := hourOf(om.Hourly.Time[i])
		day := isDaylight(hour, sunrise, sunset)
		c[i] = IconFor(om.Hourly.WeatherCode[i], day)
	}

	snap := Snapshot{
		Hi:       int(math.Round(om.Daily.ApparentTemperatureMax[0])),
		Lo:       int(math.Round(om.Daily.ApparentTemperatureMin[0])),
		Feels:    int(math.Round(om.Current.ApparentTemperature)),
		Humidity: om.Current.RelativeHumidity2m,
		Wind:     int(math.Round(om.Current.WindSpeed10m)),
		T:        t,
		C:        c,
		P:        p,
	}
	return json.Marshal(snap)
}

func parseDayBounds(om openMeteoResponse) (sunrise, sunset int) {
	sunrise, sunset = 6, 18 // fallback
	if len(om.Daily.Sunrise) > 0 {
		if h := hourOf(om.Daily.Sunrise[0]); h >= 0 {
			sunrise = h
		}
	}
	if len(om.Daily.Sunset) > 0 {
		if h := hourOf(om.Daily.Sunset[0]); h >= 0 {
			sunset = h
		}
	}
	return sunrise, sunset
}

// hourOf extracts the local hour from an Open-Meteo ISO time like "2026-10-17T14:00".
func hourOf(s string) int {
	if t, err := time.Parse("2006-01-02T15:04", s); err == nil {
		return t.Hour()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Hour()
	}
	return -1
}

func isDaylight(hour, sunrise, sunset int) bool {
	if hour < 0 {
		return true
	}
	return hour >= sunrise && hour < sunset
}

// IconFor maps a WMO weather interpretation code to a dashboard icon key.
func IconFor(code int, day bool) string {
	switch {
	case code == 0:
		if day {
			return "sun"
		}
		return "moon"
	case code == 1 || code == 2:
		return "partly"
	case code == 3 || code == 45 || code == 48:
		return "cloud"
	case code >= 51 && code <= 67:
		return "rain"
	case code >= 71 && code <= 77:
		return "rain" // no snow icon; treat as wet
	case code >= 80 && code <= 82:
		return "rain"
	case code >= 85 && code <= 86:
		return "rain"
	case code >= 95 && code <= 99:
		return "rain"
	default:
		return "cloud"
	}
}
