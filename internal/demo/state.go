package demo

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"photo-dashboard/internal/persist"
)

// Snapshot is the full dashboard payload sent to clients.
type Snapshot struct {
	People json.RawMessage  `json:"people"`
	Family json.RawMessage  `json:"family"`
	ALL    json.RawMessage  `json:"ALL"`
	Events []map[string]any `json:"events"`
	// TEMP: busyExtras was for calendar density demo; unused by UI
	BusyExtras []map[string]any   `json:"busyExtras,omitempty"`
	Routines   json.RawMessage    `json:"routines"`
	Chores     []map[string]any   `json:"chores"`
	Stars      map[string]float64 `json:"stars"`
	Week       json.RawMessage    `json:"week"`
	Stores     json.RawMessage    `json:"stores"`
	CatOrder   json.RawMessage    `json:"catOrder"`
	Groceries  []map[string]any   `json:"groceries"`
	Catalog    json.RawMessage    `json:"catalog"`
	Usuals     json.RawMessage    `json:"usuals"`
	// TEMP: rooms/devices/climate/media/cameras/scenes/agents left from removed Home/Agents views
	Rooms      json.RawMessage  `json:"rooms"`
	Devices    []map[string]any `json:"devices"`
	Climate    map[string]any   `json:"climate"`
	Media      map[string]any   `json:"media"`
	Cameras    json.RawMessage  `json:"cameras"`
	Scenes     []map[string]any `json:"scenes"`
	Weather      json.RawMessage  `json:"weather"`
	Garmin       json.RawMessage  `json:"garmin"`
	Sports       json.RawMessage  `json:"sports,omitempty"`
	Photos       []map[string]any `json:"photos"`
	Agents       json.RawMessage  `json:"agents"`
	TimeOffset      int64            `json:"timeOffset"`
	CalendarLive    bool             `json:"calendarLive"`
	CalendarPeople  []string         `json:"calendarPeople,omitempty"`
}

type Store struct {
	mu             sync.RWMutex
	snap           Snapshot
	eventsBySource map[string][]map[string]any
	nextGID        int
	nextEID        int
	nextCID        int
	db             *persist.DB
}

func Load(seedPath string) (*Store, error) {
	b, err := os.ReadFile(seedPath)
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	s := &Store{nextGID: 100, nextEID: 500, nextCID: 100, snap: Snapshot{}}
	s.snap.People = raw["people"]
	s.snap.Family = raw["family"]
	s.snap.ALL = raw["ALL"]
	s.snap.Routines = raw["routines"]
	s.snap.Week = raw["week"]
	s.snap.Stores = raw["stores"]
	s.snap.CatOrder = raw["catOrder"]
	s.snap.Catalog = raw["catalog"]
	s.snap.Usuals = raw["usuals"]
	s.snap.Rooms = raw["rooms"]
	s.snap.Cameras = raw["cameras"]
	s.snap.Weather = raw["weather"]
	s.snap.Garmin = raw["garmin"]
	s.snap.Sports = raw["sports"]
	s.snap.Agents = raw["agents"]

	mustUnmarshal(raw["events"], &s.snap.Events)
	// TEMP: busyExtras was for calendar density demo; unused by UI now
	mustUnmarshal(raw["busyExtras"], &s.snap.BusyExtras)
	mustUnmarshal(raw["devices"], &s.snap.Devices)
	mustUnmarshal(raw["climate"], &s.snap.Climate)
	mustUnmarshal(raw["media"], &s.snap.Media)
	mustUnmarshal(raw["scenes"], &s.snap.Scenes)
	mustUnmarshal(raw["photos"], &s.snap.Photos)

	s.shiftEventsToToday()
	// Real wall clock (timeOffset stays 0). setTime action kept for compatibility only.
	s.snap.TimeOffset = 0
	// Shopping/TODO come from SQLite (see AttachPersist), not seed.
	s.snap.Groceries = []map[string]any{}
	s.snap.Chores = []map[string]any{}
	s.snap.Stars = map[string]float64{}
	s.nextGID = 100
	return s, nil
}

func maxGroceryID(groceries []map[string]any) int {
	max := 100
	for _, g := range groceries {
		id, _ := g["id"].(string)
		if !strings.HasPrefix(id, "g") {
			continue
		}
		n, err := strconv.Atoi(id[1:])
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max
}

// AttachPersist binds a SQLite store and hydrates groceries/chores/stars
// and the last ingested Garmin blob (when present).
// Missing DB rows bootstrap to empty lists (no seed demo data).
func (s *Store) AttachPersist(db *persist.DB) error {
	if db == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db = db

	m, ok, err := db.LoadMutable()
	if err != nil {
		return err
	}
	if !ok {
		s.snap.Groceries = []map[string]any{}
		s.snap.Chores = []map[string]any{}
		s.snap.Stars = map[string]float64{}
		s.nextGID = 100
		if err := s.saveMutableLocked(); err != nil {
			return err
		}
	} else {
		s.snap.Groceries = m.Groceries
		s.snap.Chores = m.Chores
		s.snap.Stars = m.Stars
		s.nextGID = m.NextGID
		if s.nextGID < maxGroceryID(s.snap.Groceries) {
			s.nextGID = maxGroceryID(s.snap.Groceries)
		}
		if max := maxChoreID(s.snap.Chores); s.nextCID < max {
			s.nextCID = max
		}
	}

	garmin, found, err := db.LoadGarmin()
	if err != nil {
		return err
	}
	if found {
		s.snap.Garmin = append(json.RawMessage(nil), garmin...)
	}
	return nil
}

func maxChoreID(chores []map[string]any) int {
	max := 100
	for _, c := range chores {
		id, _ := c["id"].(string)
		if !strings.HasPrefix(id, "c") {
			continue
		}
		n, err := strconv.Atoi(id[1:])
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max
}

func (s *Store) saveMutableLocked() error {
	if s.db == nil {
		return nil
	}
	return s.db.SaveMutable(persist.Mutable{
		Groceries: s.snap.Groceries,
		Chores:    s.snap.Chores,
		Stars:     s.snap.Stars,
		NextGID:   s.nextGID,
	})
}

func (s *Store) persistAfterMutation(name string) {
	switch name {
	case "setChore", "addChore", "setGrocery", "addGrocery", "clearChecked":
		if err := s.saveMutableLocked(); err != nil {
			log.Printf("persist: save after %s: %v", name, err)
		}
	}
}

func mustUnmarshal(b json.RawMessage, v any) {
	if len(b) == 0 || string(b) == "null" {
		return
	}
	if err := json.Unmarshal(b, v); err != nil {
		panic(err)
	}
}

func (s *Store) shiftEventsToToday() {
	if len(s.snap.Events) == 0 {
		return
	}
	// Seed was generated with "today" = local calendar day of generation.
	// Re-anchor so the first all-day / earliest event day's local date == now's local date.
	first, ok := parseTime(s.snap.Events[0]["start"])
	if !ok {
		return
	}
	seedDay := time.Date(first.Year(), first.Month(), first.Day(), 0, 0, 0, 0, time.Local)
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	delta := today.Sub(seedDay)
	if delta == 0 {
		return
	}
	shift := func(list []map[string]any) {
		for _, e := range list {
			if t, ok := parseTime(e["start"]); ok {
				e["start"] = t.Add(delta).Format(time.RFC3339Nano)
			}
			if t, ok := parseTime(e["end"]); ok {
				e["end"] = t.Add(delta).Format(time.RFC3339Nano)
			}
		}
	}
	shift(s.snap.Events)
	shift(s.snap.BusyExtras)
}

func parseTime(v any) (time.Time, bool) {
	str, ok := v.(string)
	if !ok {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339Nano, str)
	if err != nil {
		t, err = time.Parse(time.RFC3339, str)
	}
	if err != nil {
		return time.Time{}, false
	}
	return t.In(time.Local), true
}

func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cloneLocked()
}

// SetWeather replaces the weather blob in the snapshot (e.g. from Open-Meteo).
func (s *Store) SetWeather(raw json.RawMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snap.Weather = append(json.RawMessage(nil), raw...)
}

// SetGarmin replaces the garmin blob in the snapshot (e.g. from the Python sidecar)
// and persists it to SQLite when attached so redeploys keep the last live data.
func (s *Store) SetGarmin(raw json.RawMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snap.Garmin = append(json.RawMessage(nil), raw...)
	if s.db == nil {
		return
	}
	if err := s.db.SaveGarmin(s.snap.Garmin); err != nil {
		log.Printf("persist: save garmin: %v", err)
	}
}

// SetSports replaces the sports fixtures blob (e.g. from TheSportsDB poller).
func (s *Store) SetSports(raw json.RawMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snap.Sports = append(json.RawMessage(nil), raw...)
}

// SetEvents replaces the gcal source events (compat wrapper for SetSourceEvents).
func (s *Store) SetEvents(events []map[string]any) {
	s.SetSourceEvents("gcal", events)
}

// SetSourceEvents replaces events for one live calendar source and rebuilds the merged list.
// Seed events are discarded once any live source is set.
func (s *Store) SetSourceEvents(source string, events []map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if source == "" {
		return
	}
	if s.eventsBySource == nil {
		s.eventsBySource = map[string][]map[string]any{}
	}
	cp := make([]map[string]any, len(events))
	for i, e := range events {
		m := make(map[string]any, len(e))
		for k, v := range e {
			m[k] = v
		}
		cp[i] = m
	}
	s.eventsBySource[source] = cp
	s.rebuildEventsLocked()
	s.snap.CalendarLive = true
}

func (s *Store) rebuildEventsLocked() {
	order := []string{"gcal", "pmday"}
	seen := map[string]bool{}
	var out []map[string]any
	for _, src := range order {
		if evs, ok := s.eventsBySource[src]; ok {
			out = append(out, evs...)
			seen[src] = true
		}
	}
	for src, evs := range s.eventsBySource {
		if seen[src] {
			continue
		}
		out = append(out, evs...)
	}
	s.snap.Events = out
}

// SetCalendarPeople sets which person ids get personal calendar lanes (plus Family in the UI).
func (s *Store) SetCalendarPeople(ids []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snap.CalendarPeople = append([]string(nil), ids...)
}

// PatchSourceEvent updates fields on a single event within a source by dashboard id.
func (s *Store) PatchSourceEvent(source, id string, patch map[string]any) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.eventsBySource == nil {
		return false
	}
	evs := s.eventsBySource[source]
	for _, e := range evs {
		if e["id"] != id {
			continue
		}
		for k, v := range patch {
			e[k] = v
		}
		s.rebuildEventsLocked()
		return true
	}
	return false
}

// CalendarLive reports whether events are driven by a live calendar sync.
func (s *Store) CalendarLive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snap.CalendarLive
}

func (s *Store) cloneLocked() Snapshot {
	b, _ := json.Marshal(s.snap)
	var out Snapshot
	_ = json.Unmarshal(b, &out)
	return out
}

func (s *Store) setTimeLocked(h, m int) {
	now := time.Now()
	target := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
	s.snap.TimeOffset = target.UnixMilli() - now.UnixMilli()
}

// Apply runs a named action and returns the new snapshot.
func (s *Store) Apply(name string, payload map[string]any) (Snapshot, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	eventName := ""
	switch name {
	case "setTime":
		// Kept for compatibility; UI no longer uses fake clock presets.
		if payload["real"] == true {
			s.snap.TimeOffset = 0
		} else {
			h := intNum(payload["h"])
			m := intNum(payload["m"])
			s.setTimeLocked(h, m)
		}
	case "setChore":
		s.setChore(payload)
	case "addChore":
		s.addChore(payload)
	case "setGrocery":
		s.setGrocery(payload)
	case "addGrocery":
		s.addGrocery(payload)
	case "clearChecked":
		kept := s.snap.Groceries[:0]
		for _, g := range s.snap.Groceries {
			if g["done"] != true {
				kept = append(kept, g)
			}
		}
		s.snap.Groceries = kept
	case "setDevice":
		s.setDevice(payload)
	case "applyScene":
		s.applyScene(payload)
	case "setClimate":
		for k, v := range payload {
			if k == "id" {
				continue
			}
			s.snap.Climate[k] = v
		}
	case "setMedia":
		for k, v := range payload {
			s.snap.Media[k] = v
		}
	case "addEvent":
		if s.snap.CalendarLive {
			// Read-only while Google Calendar sync is active.
			break
		}
		s.addEvent(payload)
	case "removeEvent":
		if s.snap.CalendarLive {
			break
		}
		id, _ := payload["id"].(string)
		filtered := s.snap.Events[:0]
		for _, e := range s.snap.Events {
			if e["id"] != id {
				filtered = append(filtered, e)
			}
		}
		s.snap.Events = filtered
	case "toggleFav":
		seed, _ := payload["seed"].(string)
		for _, p := range s.snap.Photos {
			if p["seed"] == seed {
				fav, _ := p["fav"].(bool)
				p["fav"] = !fav
				break
			}
		}
	case "alert":
		if n, ok := payload["name"].(string); ok {
			eventName = n
			if n == "garage" {
				s.patchDevice("g-door", map[string]any{"state": "open"})
			}
		}
	default:
		// unknown — still return snapshot
	}
	s.persistAfterMutation(name)
	return s.cloneLocked(), eventName, nil
}

func (s *Store) setChore(p map[string]any) {
	id, _ := p["id"].(string)
	done, _ := p["done"].(bool)
	doneBy, _ := p["doneBy"].(string)
	for _, c := range s.snap.Chores {
		if c["id"] != id {
			continue
		}
		wasDone, _ := c["done"].(bool)
		stars, _ := c["stars"].(float64)
		who, _ := c["who"].(string)
		prevBy, _ := c["doneBy"].(string)

		earner := ""
		if _, ok := s.snap.Stars[who]; ok {
			earner = who
		} else if doneBy != "" {
			if _, ok := s.snap.Stars[doneBy]; ok {
				earner = doneBy
			}
		} else if prevBy != "" {
			if _, ok := s.snap.Stars[prevBy]; ok {
				earner = prevBy
			}
		}

		if wasDone && !done {
			c["done"] = false
			delete(c, "doneBy")
			if earner != "" && stars > 0 {
				s.snap.Stars[earner] -= stars
			}
		} else if !wasDone && done {
			c["done"] = true
			if doneBy != "" {
				c["doneBy"] = doneBy
				if _, ok := s.snap.Stars[doneBy]; ok {
					earner = doneBy
				}
			}
			if earner != "" && stars > 0 {
				s.snap.Stars[earner] += stars
			}
		}
		return
	}
}

func (s *Store) addChore(p map[string]any) {
	title, _ := p["title"].(string)
	title = strings.TrimSpace(title)
	if title == "" {
		return
	}
	who, _ := p["who"].(string)
	if who == "" {
		who = "any"
	}
	r, _ := p["r"].(string)
	if r == "" {
		r = "morning"
	}
	emoji, _ := p["emoji"].(string)
	if emoji == "" {
		emoji = "✓"
	}
	stars := float64(intNum(p["stars"]))
	if stars < 0 {
		stars = 0
	}
	day, _ := p["day"].(string)
	day = strings.TrimSpace(day)
	if day == "" {
		day = time.Now().Format("2006-01-02")
	} else if _, err := time.Parse("2006-01-02", day); err != nil {
		day = time.Now().Format("2006-01-02")
	}
	if max := maxChoreID(s.snap.Chores); s.nextCID < max {
		s.nextCID = max
	}
	s.nextCID++
	s.snap.Chores = append(s.snap.Chores, map[string]any{
		"id":    "c" + itoa(s.nextCID),
		"title": title,
		"who":   who,
		"r":     r,
		"emoji": emoji,
		"stars": stars,
		"day":   day,
		"done":  false,
	})
}

func (s *Store) setGrocery(p map[string]any) {
	id, _ := p["id"].(string)
	for _, g := range s.snap.Groceries {
		if g["id"] != id {
			continue
		}
		if v, ok := p["done"]; ok {
			g["done"] = v
		}
		if v, ok := p["qty"]; ok {
			g["qty"] = v
		}
		return
	}
}

func (s *Store) addGrocery(p map[string]any) {
	name, _ := p["name"].(string)
	if name == "" {
		return
	}
	// bump qty or revive
	for _, g := range s.snap.Groceries {
		n, _ := g["name"].(string)
		if equalFold(n, name) && g["done"] != true {
			qty := intNum(g["qty"])
			g["qty"] = qty + 1
			return
		}
		if equalFold(n, name) && g["done"] == true {
			g["done"] = false
			g["qty"] = 1
			return
		}
	}
	s.nextGID++
	cat, _ := p["cat"].(string)
	if cat == "" {
		cat = "Other"
	}
	store, _ := p["store"].(string)
	if store == "" {
		store = "grocer"
	}
	s.snap.Groceries = append(s.snap.Groceries, map[string]any{
		"id":    "g" + itoa(s.nextGID),
		"name":  name,
		"cat":   cat,
		"qty":   1,
		"store": store,
		"done":  false,
		"note":  "",
	})
}

func (s *Store) setDevice(p map[string]any) {
	id, _ := p["id"].(string)
	patch, _ := p["patch"].(map[string]any)
	if patch == nil {
		patch = map[string]any{}
		for k, v := range p {
			if k != "id" && k != "patch" {
				patch[k] = v
			}
		}
	}
	s.patchDevice(id, patch)
}

func (s *Store) patchDevice(id string, patch map[string]any) {
	for _, d := range s.snap.Devices {
		if d["id"] != id {
			continue
		}
		for k, v := range patch {
			d[k] = v
		}
		return
	}
}

func (s *Store) applyScene(p map[string]any) {
	id, _ := p["id"].(string)
	for _, sc := range s.snap.Scenes {
		if sc["id"] != id {
			continue
		}
		set, _ := sc["set"].(map[string]any)
		for devID, level := range set {
			lvl := intNum(level)
			for _, d := range s.snap.Devices {
				if d["id"] != devID {
					continue
				}
				t, _ := d["type"].(string)
				if t == "light" {
					if lvl <= 0 {
						d["on"] = false
					} else {
						d["on"] = true
						d["level"] = lvl
					}
				} else if t == "blind" {
					d["pos"] = lvl
				}
			}
		}
		return
	}
}

func (s *Store) addEvent(p map[string]any) {
	s.nextEID++
	e := map[string]any{"id": "e" + itoa(s.nextEID)}
	for k, v := range p {
		if k == "id" {
			continue
		}
		e[k] = v
	}
	s.snap.Events = append(s.snap.Events, e)
}

func intNum(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}

func equalFold(a, b string) bool {
	return strings.EqualFold(a, b)
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

// SeedPath resolves seed.json next to the binary or from known relative paths.
func SeedPath() string {
	candidates := []string{
		"internal/demo/seed.json",
		filepath.Join("..", "..", "internal", "demo", "seed.json"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "internal/demo/seed.json"
}
