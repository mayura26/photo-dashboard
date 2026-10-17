package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"photo-dashboard/internal/demo"
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

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.ServeWS)

	static, err := webfs.FS()
	if err != nil {
		log.Fatal("web fs: ", err)
	}
	mux.Handle("/", spaFileServer(static))

	log.Printf("Home HQ listening on http://localhost%s", addr)
	log.Printf("WebSocket at ws://localhost%s/ws", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
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
