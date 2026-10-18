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

	"photo-dashboard/internal/auth"
	"photo-dashboard/internal/demo"
	"photo-dashboard/internal/gcal"
	"photo-dashboard/internal/persist"
	"photo-dashboard/internal/photos"
	"photo-dashboard/internal/pmday"
	"photo-dashboard/internal/sports"
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

	dbPath := strings.TrimSpace(os.Getenv("HQ_DB_PATH"))
	if dbPath == "" {
		dbPath = filepath.Join("data", "hq.db")
	}
	db, err := persist.Open(dbPath)
	if err != nil {
		log.Fatal("persist open: ", err)
	}
	defer db.Close()
	if err := store.AttachPersist(db); err != nil {
		log.Fatal("persist hydrate: ", err)
	}
	log.Printf("persist: shopping/TODO at %s", dbPath)

	static, err := webfs.FS()
	if err != nil {
		log.Fatal("web fs: ", err)
	}
	version := resolveAppVersion(static)
	hub := demo.NewHub(store, version)

	startWeatherPoller(store, hub)
	startGCalPoller(store, hub)
	startPmdayPoller(store, hub)
	startSportsPoller(store, hub)

	gate := auth.FromEnv()
	if gate.Enabled() {
		log.Printf("auth: HQ_PASSWORD set; dashboard login required")
	} else {
		log.Printf("auth: HQ_PASSWORD unset; dashboard open")
	}

	photoDir := photos.DirFromDBPath(dbPath)
	if err := photoDir.Ensure(); err != nil {
		log.Fatal("photos dir: ", err)
	}
	photoAPI := &photos.API{Dir: photoDir, Lib: store, Hub: hub}
	log.Printf("photos: library at %s", photoDir.Path)
	if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) != "" {
		log.Printf("photos: OPENAI_API_KEY set; GPT-5.4 place fallback enabled")
	} else {
		log.Printf("photos: OPENAI_API_KEY unset; place only from GPS EXIF")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", gate.RequireFunc(hub.ServeWS))
	mux.HandleFunc("/api/auth/status", gate.HandleStatus)
	mux.HandleFunc("/api/login", gate.HandleLogin)
	mux.HandleFunc("/api/logout", gate.HandleLogout)
	mux.HandleFunc("/internal/garmin", garminIngest(store, hub))
	mux.HandleFunc("/api/photos", gate.RequireFunc(photoAPI.HandleCollection))
	mux.HandleFunc("/api/photos/", gate.RequireFunc(photoAPI.HandleItem))
	mux.HandleFunc("/media/photos/", gate.RequireFunc(photoAPI.HandleMedia))
	mux.HandleFunc("/version.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"version": version})
	})
	// Static SPA is public so the login gate can load; data lives behind /ws.
	mux.Handle("/", spaFileServer(static))

	log.Printf("Home HQ listening on http://localhost%s", addr)
	log.Printf("WebSocket at ws://localhost%s/ws", addr)
	log.Printf("App version %s", version)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// resolveAppVersion prefers APP_VERSION, then SOURCE_COMMIT (Coolify),
// then web/version.json, then "dev". Long SHAs are shortened to 7 chars.
func resolveAppVersion(webRoot fs.FS) string {
	if v := shortVersion(os.Getenv("APP_VERSION")); v != "" {
		return v
	}
	if v := shortVersion(os.Getenv("SOURCE_COMMIT")); v != "" {
		return v
	}
	raw, err := fs.ReadFile(webRoot, "version.json")
	if err != nil {
		return "dev"
	}
	var meta struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return "dev"
	}
	if v := shortVersion(meta.Version); v != "" {
		return v
	}
	return "dev"
}

func shortVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	// Git SHA (hex) → short form; leave tags/other labels alone if short.
	if len(v) > 7 && isHex(v) {
		return v[:7]
	}
	return v
}

func isHex(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
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

func startSportsPoller(store *demo.Store, hub *demo.Hub) {
	watchRaw := strings.TrimSpace(os.Getenv("SPORTS_WATCH"))
	if watchRaw == "" {
		log.Printf("sports: SPORTS_WATCH unset; using placeholder fixtures")
		return
	}
	items, err := sports.ParseWatchlist(watchRaw)
	if err != nil {
		log.Printf("sports: invalid SPORTS_WATCH %q: %v; using placeholders", watchRaw, err)
		return
	}

	apiKey := strings.TrimSpace(os.Getenv("SPORTS_API_KEY"))
	client := sports.NewClient(apiKey)
	resolved, err := sports.Resolve(client, items)
	if err != nil {
		log.Printf("sports: resolve failed: %v; using placeholders", err)
		return
	}

	interval := 30 * time.Minute
	if v := os.Getenv("SPORTS_REFRESH"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			log.Printf("sports: invalid SPORTS_REFRESH %q; using %s", v, interval)
		} else if d > 0 {
			interval = d
		}
	}

	p := &sports.Poller{
		Client:   client,
		Watches:  resolved,
		Interval: interval,
		Store:    store,
		Hub:      hub,
	}
	p.Start()
	names := make([]string, len(resolved))
	for i, r := range resolved {
		if r.Kind == sports.KindTeam {
			names[i] = "team:" + r.TeamName
		} else {
			names[i] = "league:" + r.League
		}
	}
	log.Printf("sports: TheSportsDB poller every %s for %s", interval, strings.Join(names, ", "))
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

	mayuraID := os.Getenv("GCAL_MAYURA")
	familyID := os.Getenv("GCAL_FAMILY")
	// GCAL_DENISE is intentionally unused: calendar lanes are Mayura + Family only.
	if mayuraID == "" && familyID == "" {
		log.Printf("gcal: no GCAL_MAYURA/GCAL_FAMILY; using seed calendar")
		return
	}

	var calPeople []string
	var cals []gcal.CalendarRef
	if mayuraID != "" {
		calPeople = append(calPeople, "mayura")
		cals = append(cals, gcal.CalendarRef{ID: mayuraID, Source: "personal", People: []string{"mayura"}})
	}
	if familyID != "" {
		famPeople := []string{"mayura", "denise"}
		cals = append(cals, gcal.CalendarRef{ID: familyID, Source: "family", People: famPeople})
	}
	if len(calPeople) > 0 {
		store.SetCalendarPeople(calPeople)
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

func startPmdayPoller(store *demo.Store, hub *demo.Hub) {
	baseURL := strings.TrimSpace(os.Getenv("PLAN_MY_DAY_URL"))
	if baseURL == "" {
		baseURL = "https://plan-my-day.edgeanalytics.app"
	}
	apiKey := strings.TrimSpace(os.Getenv("PLAN_MY_DAY_API_KEY"))
	if apiKey == "" {
		log.Printf("pmday: PLAN_MY_DAY_API_KEY unset; Plan My Day sync disabled")
		return
	}

	client, err := pmday.NewClient(baseURL, apiKey)
	if err != nil {
		log.Printf("pmday: client init failed: %v", err)
		return
	}

	interval := 15 * time.Minute
	if v := os.Getenv("PLAN_MY_DAY_REFRESH"); v != "" {
		if d, err := parseFlexDuration(v); err != nil {
			log.Printf("pmday: invalid PLAN_MY_DAY_REFRESH %q; using %s", v, interval)
		} else if d > 0 {
			interval = d
		}
	}
	past := 7 * 24 * time.Hour
	if v := os.Getenv("GCAL_WINDOW_PAST"); v != "" {
		if d, err := parseFlexDuration(v); err == nil && d > 0 {
			past = d
		}
	}
	future := 60 * 24 * time.Hour
	if v := os.Getenv("GCAL_WINDOW_FUTURE"); v != "" {
		if d, err := parseFlexDuration(v); err == nil && d > 0 {
			future = d
		}
	}

	store.SetCalendarPeople([]string{"mayura"})

	actions := &pmday.Actions{
		Client: client,
		Store:  store,
		Past:   past,
		Future: future,
	}
	hub.SetPmdActions(actions)

	p := &pmday.Poller{
		Client:   client,
		Interval: interval,
		Past:     past,
		Future:   future,
		Store:    store,
		Hub:      hub,
	}
	p.Start()
	log.Printf("pmday: poller every %s (%s, window -%s / +%s)", interval, baseURL, past, future)
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
		if shouldNoCache(path) {
			w.Header().Set("Cache-Control", "no-cache")
		}
		fileServer.ServeHTTP(w, r)
	})
}

func shouldNoCache(path string) bool {
	switch path {
	case "index.html", "version.json":
		return true
	}
	return strings.HasSuffix(path, ".html") ||
		strings.HasSuffix(path, ".js") ||
		strings.HasSuffix(path, ".css")
}
