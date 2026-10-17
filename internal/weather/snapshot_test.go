package weather

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestIconFor(t *testing.T) {
	cases := []struct {
		code int
		day  bool
		want string
	}{
		{0, true, "sun"},
		{0, false, "moon"},
		{1, true, "partly"},
		{2, false, "partly"},
		{3, true, "cloud"},
		{45, true, "cloud"},
		{61, true, "rain"},
		{71, true, "rain"},
		{95, false, "rain"},
	}
	for _, tc := range cases {
		if got := IconFor(tc.code, tc.day); got != tc.want {
			t.Errorf("IconFor(%d, %v) = %q, want %q", tc.code, tc.day, got, tc.want)
		}
	}
}

func TestBuildSnapshot(t *testing.T) {
	om := openMeteoResponse{}
	om.Current.ApparentTemperature = -1.4
	om.Current.RelativeHumidity2m = 70
	om.Current.WindSpeed10m = 12.6
	om.Daily.ApparentTemperatureMax = []float64{17.2}
	om.Daily.ApparentTemperatureMin = []float64{8.1}
	om.Daily.Sunrise = []string{"2026-10-17T07:12"}
	om.Daily.Sunset = []string{"2026-10-17T18:45"}

	om.Hourly.Time = make([]string, 24)
	om.Hourly.ApparentTemperature = make([]float64, 24)
	om.Hourly.WeatherCode = make([]int, 24)
	om.Hourly.Showers = make([]float64, 24)
	for i := 0; i < 24; i++ {
		om.Hourly.Time[i] = "2026-10-17T" + pad2(i) + ":00"
		om.Hourly.ApparentTemperature[i] = float64(10 + i%5)
		om.Hourly.WeatherCode[i] = 0
		om.Hourly.Showers[i] = 0
	}
	om.Hourly.Showers[10] = 0.14
	om.Hourly.Showers[16] = 1.26

	raw, err := BuildSnapshot(om)
	if err != nil {
		t.Fatal(err)
	}
	var snap Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Hi != 17 || snap.Lo != 8 {
		t.Errorf("hi/lo = %d/%d, want 17/8", snap.Hi, snap.Lo)
	}
	if snap.Feels != -1 || snap.Humidity != 70 || snap.Wind != 13 {
		t.Errorf("current fields: feels=%d humidity=%d wind=%d", snap.Feels, snap.Humidity, snap.Wind)
	}
	if len(snap.T) != 24 || len(snap.C) != 24 || len(snap.P) != 24 {
		t.Fatalf("array lengths t=%d c=%d p=%d", len(snap.T), len(snap.C), len(snap.P))
	}
	if snap.C[0] != "moon" {
		t.Errorf("hour 0 clear night = %q, want moon", snap.C[0])
	}
	if snap.C[12] != "sun" {
		t.Errorf("hour 12 clear day = %q, want sun", snap.C[12])
	}
	if snap.P[10] != 0.1 {
		t.Errorf("p[10] = %v, want 0.1", snap.P[10])
	}
	if snap.P[16] != 1.3 {
		t.Errorf("p[16] = %v, want 1.3", snap.P[16])
	}
	if snap.T[3] != 13 {
		t.Errorf("t[3] = %d, want 13 (apparent)", snap.T[3])
	}
}

func pad2(n int) string {
	return fmt.Sprintf("%02d", n)
}
