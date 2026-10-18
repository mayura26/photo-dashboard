package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"photo-dashboard/internal/demo"
	"photo-dashboard/internal/gcal"
	"photo-dashboard/internal/weather"
	"photo-dashboard/internal/webfs"
)

func main() {
	addr := ":8080"
	if v := os.Getenv("ADDR"); v != "" {
		addr = v
	}

	seed := filepath.Join(webfs.ModuleRoot(), "internal", "demo", "seed.json")
	if _, err := os.Stat(seed); err != nil {
		seed = demo.SeedPath()
	}
	store, err := demo.Load(seed)
	if err != nil {
		log.Fatal("load seed: ", err)
	}
	hub := demo.NewHub(store)

	startWeatherPoller(store, hub)
	startGCalPoller(store, hub)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.ServeWS)
	mux.HandleFunc("/internal/garmin", garminIngest(store, hub))

	static, err := webfs.FS()
	if err != nil {
		log.Fatal("web fs: ", err)
	}
	mux.Handle("/", spaFileServer(static))

	log.Printf("Home HQ listening on http://localhost%s", addr)
	log.Printf("WebSocket at ws://localhost%s/ws", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func garminIngest(store *demo.Store, hub *demo.Hub) http.HandlerFunc {
	token := os.Getenv("HQ_INGEST_TOKEN")
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if token == "" || r.Header.Get("X-HQ-Token") != token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		if !json.Valid(body) {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		store.SetGarmin(json.RawMessage(body))
		hub.BroadcastSnapshot()
		w.WriteHeader(http.StatusNoContent)
		log.Printf("garmin: snapshot updated (%d bytes)", len(body))
	}
}

func startWeatherPoller(store *demo.Store, hub *demo.Hub) {
	latS, lonS := os.Getenv("WEATHER_LAT"), os.Getenv("WEATHER_LON")
	if latS == "" || lonS == "" {
		log.Printf("weather: WEATHER_LAT/WEATHER_LON unset; using seed weather")
		return
	}
	lat, err1 := strconv.ParseFloat(latS, 64)
	lon, err2 := strconv.ParseFloat(lonS, 64)
	if err1 != nil || err2 != nil {
		log.Printf("weather: invalid WEATHER_LAT/WEATHER_LON; using seed weather")
		return
	}

	interval := 15 * time.Minute
	if v := os.Getenv("WEATHER_REFRESH"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			log.Printf("weather: invalid WEATHER_REFRESH %q; using %s", v, interval)
		} else if d > 0 {
			interval = d
		}
	}

	p := &weather.Poller{
		Client:   weather.NewClient(),
		Lat:      lat,
		Lon:      lon,
		Interval: interval,
		Store:    store,
		Hub:      hub,
	}
	p.Start()
	log.Printf("weather: Open-Meteo poller every %s for %.4f, %.4f", interval, lat, lon)
}

func startGCalPoller(store *demo.Store, hub *demo.Hub) {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	refresh := os.Getenv("GOOGLE_REFRESH_TOKEN")
	if clientID == "" || clientSecret == "" || refresh == "" {
		log.Printf("gcal: OAuth env unset; using seed calendar")
		return
	}

	alexID := os.Getenv("GCAL_ALEX")
	jordanID := os.Getenv("GCAL_JORDAN")
	familyID := os.Getenv("GCAL_FAMILY")
	if alexID == "" && jordanID == "" && familyID == "" {
		log.Printf("gcal: no GCAL_ALEX/GCAL_JORDAN/GCAL_FAMILY; using seed calendar")
		return
	}

	var people []string
	var cals []gcal.CalendarRef
	if alexID != "" {
		people = append(people, "alex")
		cals = append(cals, gcal.CalendarRef{ID: alexID, Source: "personal", People: []string{"alex"}})
	}
	if jordanID != "" {
		people = append(people, "jordan")
		cals = append(cals, gcal.CalendarRef{ID: jordanID, Source: "personal", People: []string{"jordan"}})
	}
	if familyID != "" {
		famPeople := people
		if len(famPeople) == 0 {
			famPeople = []string{"alex", "jordan"}
		}
		cals = append(cals, gcal.CalendarRef{ID: familyID, Source: "family", People: append([]string(nil), famPeople...)})
	}

	ctx := context.Background()
	client, err := gcal.NewClient(ctx, clientID, clientSecret, refresh)
	if err != nil {
		log.Printf("gcal: client init failed: %v; using seed calendar", err)
		return
	}

	interval := 15 * time.Minute
	if v := os.Getenv("GCAL_REFRESH"); v != "" {
		if d, err := parseFlexDuration(v); err != nil {
			log.Printf("gcal: invalid GCAL_REFRESH %q; using %s", v, interval)
		} else if d > 0 {
			interval = d
		}
	}
	past := 7 * 24 * time.Hour
	if v := os.Getenv("GCAL_WINDOW_PAST"); v != "" {
		if d, err := parseFlexDuration(v); err != nil {
			log.Printf("gcal: invalid GCAL_WINDOW_PAST %q; using %s", v, past)
		} else if d > 0 {
			past = d
		}
	}
	future := 60 * 24 * time.Hour
	if v := os.Getenv("GCAL_WINDOW_FUTURE"); v != "" {
		if d, err := parseFlexDuration(v); err != nil {
			log.Printf("gcal: invalid GCAL_WINDOW_FUTURE %q; using %s", v, future)
		} else if d > 0 {
			future = d
		}
	}

	p := &gcal.Poller{
		Client:   client,
		Cals:     cals,
		Interval: interval,
		Past:     past,
		Future:   future,
		Store:    store,
		Hub:      hub,
	}
	p.Start()
	log.Printf("gcal: poller every %s (%d calendar(s), window -%s / +%s)", interval, len(cals), past, future)
}

// parseFlexDuration accepts Go durations plus a trailing "d" for days (e.g. 7d, 15m).
func parseFlexDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if strings.HasSuffix(s, "d") && len(s) > 1 {
		n := s[:len(s)-1]
		ok := true
		for _, r := range n {
			if !unicode.IsDigit(r) {
				ok = false
				break
			}
		}
		if ok {
			days, err := strconv.Atoi(n)
			if err != nil {
				return 0, err
			}
			return time.Duration(days) * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(s)
}

func spaFileServer(root fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(root, path); err != nil {
			// fall through to FileServer for 404 on missing assets
		}
		fileServer.ServeHTTP(w, r)
	})
}
