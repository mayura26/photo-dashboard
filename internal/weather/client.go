package weather

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const defaultBaseURL = "https://api.open-meteo.com/v1/forecast"

// Client fetches forecasts from Open-Meteo.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient() *Client {
	return &Client{
		BaseURL: defaultBaseURL,
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// openMeteoResponse matches the subset of fields we need from the forecast API.
type openMeteoResponse struct {
	Current struct {
		Temperature2m       float64 `json:"temperature_2m"`
		ApparentTemperature float64 `json:"apparent_temperature"`
		RelativeHumidity2m  int     `json:"relative_humidity_2m"`
		WindSpeed10m        float64 `json:"wind_speed_10m"`
		WeatherCode         int     `json:"weather_code"`
		IsDay               int     `json:"is_day"`
	} `json:"current"`
	Hourly struct {
		Time                     []string  `json:"time"`
		Temperature2m            []float64 `json:"temperature_2m"`
		WeatherCode              []int     `json:"weather_code"`
		PrecipitationProbability []float64 `json:"precipitation_probability"`
	} `json:"hourly"`
	Daily struct {
		Temperature2mMax []float64 `json:"temperature_2m_max"`
		Temperature2mMin []float64 `json:"temperature_2m_min"`
		Sunrise          []string  `json:"sunrise"`
		Sunset           []string  `json:"sunset"`
	} `json:"daily"`
}

// Fetch returns dashboard weather JSON for the given coordinates.
func (c *Client) Fetch(lat, lon float64) (json.RawMessage, error) {
	base := c.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	hc := c.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}

	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(lat, 'f', 4, 64))
	q.Set("longitude", strconv.FormatFloat(lon, 'f', 4, 64))
	q.Set("timezone", "auto")
	q.Set("forecast_days", "1")
	q.Set("current", "temperature_2m,apparent_temperature,relative_humidity_2m,wind_speed_10m,weather_code,is_day")
	q.Set("hourly", "temperature_2m,weather_code,precipitation_probability")
	q.Set("daily", "temperature_2m_max,temperature_2m_min,sunrise,sunset")

	reqURL := base + "?" + q.Encode()
	resp, err := hc.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("open-meteo request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("open-meteo read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open-meteo status %d: %s", resp.StatusCode, truncate(body, 200))
	}

	var om openMeteoResponse
	if err := json.Unmarshal(body, &om); err != nil {
		return nil, fmt.Errorf("open-meteo decode: %w", err)
	}
	return BuildSnapshot(om)
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
