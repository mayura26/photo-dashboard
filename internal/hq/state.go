package hq

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
	People         json.RawMessage    `json:"people"`
	Family         json.RawMessage    `json:"family"`
	ALL            json.RawMessage    `json:"ALL"`
	Events         []map[string]any   `json:"events"`
	Routines       json.RawMessage    `json:"routines"`
	Chores         []map[string]any   `json:"chores"`
	Stars          map[string]float64 `json:"stars"`
	Week           json.RawMessage    `json:"week"`
	Stores         json.RawMessage    `json:"stores"`
	CatOrder       json.RawMessage    `json:"catOrder"`
	Groceries      []map[string]any   `json:"groceries"`
	Catalog        json.RawMessage    `json:"catalog"`
	Usuals         json.RawMessage    `json:"usuals"`
	Weather        json.RawMessage    `json:"weather,omitempty"`
	Garmin         json.RawMessage    `json:"garmin,omitempty"`
	Sports         json.RawMessage    `json:"sports,omitempty"`
	Photos         []map[string]any   `json:"photos"`
	Payouts        []map[string]any   `json:"payouts"`
	PayoutFirms    []map[string]any   `json:"payoutFirms"`
	TimeOffset     int64              `json:"timeOffset"`
	CalendarLive   bool               `json:"calendarLive"`
	CalendarPeople []string           `json:"calendarPeople,omitempty"`
}

type Store struct {
	mu             sync.RWMutex
	snap           Snapshot
	eventsBySource map[string][]map[string]any
	nextGID        int
	nextEID        int
	nextCID        int
	db             *persist.DB
	weekHits       map[string]map[string]bool
	choreDay       string // last local day ensureChoreDayLocked ran for
}

// Load reads household.json (people, routines, shopping catalog) into a Store.
// Live fields (events, weather, garmin, sports, groceries, chores, photos) start empty
// and are filled by pollers / AttachPersist.
func Load(householdPath string) (*Store, error) {
	b, err := os.ReadFile(householdPath)
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	s := &Store{
		nextGID:  100,
		nextEID:  500,
		nextCID:  100,
		snap:     Snapshot{},
		weekHits: map[string]map[string]bool{},
	}
	s.snap.People = raw["people"]
	s.snap.Family = raw["family"]
	s.snap.ALL = raw["ALL"]
	s.snap.Routines = raw["routines"]
	s.snap.Stores = raw["stores"]
	s.snap.CatOrder = raw["catOrder"]
	s.snap.Catalog = raw["catalog"]
	s.snap.Usuals = raw["usuals"]

	s.snap.Events = []map[string]any{}
	s.snap.Photos = []map[string]any{}
	s.snap.Payouts = []map[string]any{}
	s.snap.PayoutFirms = []map[string]any{}
	s.snap.Groceries = []map[string]any{}
	s.snap.Chores = []map[string]any{}
	s.snap.Stars = map[string]float64{}
	s.snap.TimeOffset = 0
	s.recomputeWeekLocked(todayKey())
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
// Missing DB rows bootstrap to empty lists.
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
		s.weekHits = map[string]map[string]bool{}
		s.nextGID = 100
		if err := s.saveMutableLocked(); err != nil {
			return err
		}
	} else {
		s.snap.Groceries = m.Groceries
		s.snap.Chores = m.Chores
		s.snap.Stars = m.Stars
		s.weekHits = m.WeekHits
		if s.weekHits == nil {
			s.weekHits = map[string]map[string]bool{}
		}
		s.nextGID = m.NextGID
		if s.nextGID < maxGroceryID(s.snap.Groceries) {
			s.nextGID = maxGroceryID(s.snap.Groceries)
		}
		if max := maxChoreID(s.snap.Chores); s.nextCID < max {
			s.nextCID = max
		}
	}

	if s.ensureChoreDayLocked() {
		if err := s.saveMutableLocked(); err != nil {
			return err
		}
	}

	garmin, found, err := db.LoadGarmin()
	if err != nil {
		return err
	}
	if found {
		s.snap.Garmin = append(json.RawMessage(nil), garmin...)
	}

	photos, found, err := db.LoadPhotos()
	if err != nil {
		return err
	}
	if found {
		s.snap.Photos = photos
	} else {
		s.snap.Photos = []map[string]any{}
		if err := s.savePhotosLocked(); err != nil {
			return err
		}
	}

	payouts, firms, found, err := db.LoadPayouts()
	if err != nil {
		return err
	}
	if found {
		s.snap.Payouts = payouts
		s.snap.PayoutFirms = firms
	} else {
		s.snap.Payouts = []map[string]any{}
		s.snap.PayoutFirms = []map[string]any{}
		if err := s.savePayoutsLocked(); err != nil {
			return err
		}
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
		WeekHits:  s.weekHits,
		NextGID:   s.nextGID,
	})
}

func (s *Store) savePhotosLocked() error {
	if s.db == nil {
		return nil
	}
	return s.db.SavePhotos(s.snap.Photos)
}

func (s *Store) savePayoutsLocked() error {
	if s.db == nil {
		return nil
	}
	if s.snap.Payouts == nil {
		s.snap.Payouts = []map[string]any{}
	}
	if s.snap.PayoutFirms == nil {
		s.snap.PayoutFirms = []map[string]any{}
	}
	return s.db.SavePayouts(s.snap.Payouts, s.snap.PayoutFirms)
}

func (s *Store) persistAfterMutation(name string) {
	switch name {
	case "setChore", "addChore", "delayChore", "setGrocery", "addGrocery", "clearChecked":
		if err := s.saveMutableLocked(); err != nil {
			log.Printf("persist: save after %s: %v", name, err)
		}
	case "toggleFav":
		if err := s.savePhotosLocked(); err != nil {
			log.Printf("persist: save photos after %s: %v", name, err)
		}
	}
}

func (s *Store) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ensureChoreDayLocked() {
		if err := s.saveMutableLocked(); err != nil {
			log.Printf("persist: save after chore day roll: %v", err)
		}
	}
	return s.cloneLocked()
}

// DefaultPhotoFrom returns the first person id for new uploads.
func (s *Store) DefaultPhotoFrom() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.personIDsLocked()
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

// AddPhoto appends a photo record and persists it.
func (s *Store) AddPhoto(p map[string]any) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snap.Photos = append(s.snap.Photos, p)
	if err := s.savePhotosLocked(); err != nil {
		s.snap.Photos = s.snap.Photos[:len(s.snap.Photos)-1]
		return nil, err
	}
	return cloneMap(p), nil
}

// PhotoList returns a shallow clone of photo metadata maps.
func (s *Store) PhotoList() []map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]map[string]any, len(s.snap.Photos))
	for i, p := range s.snap.Photos {
		out[i] = cloneMap(p)
	}
	return out
}

// FindPhotoByHash returns the first photo with the given content hash.
func (s *Store) FindPhotoByHash(hash string) (map[string]any, bool) {
	if hash == "" {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.snap.Photos {
		h, _ := p["hash"].(string)
		if h == hash {
			return cloneMap(p), true
		}
	}
	return nil, false
}

// SetPhotoHashes sets hash fields for the given photo ids and persists once.
func (s *Store) SetPhotoHashes(hashes map[string]string) error {
	if len(hashes) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for _, p := range s.snap.Photos {
		id, _ := p["id"].(string)
		h, ok := hashes[id]
		if !ok || h == "" {
			continue
		}
		p["hash"] = h
		changed = true
	}
	if !changed {
		return nil
	}
	return s.savePhotosLocked()
}

// SetPhotoFits stores the frame ids a photo survives and persists.
// Missing photos return found=false. fits is copied so the caller can reuse the slice.
func (s *Store) SetPhotoFits(id string, fits []string) (map[string]any, bool, error) {
	if id == "" || len(fits) == 0 {
		return nil, false, nil
	}
	cp := append([]string(nil), fits...)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.snap.Photos {
		if p["id"] != id {
			continue
		}
		p["fits"] = cp
		if err := s.savePhotosLocked(); err != nil {
			return nil, true, err
		}
		return cloneMap(p), true, nil
	}
	return nil, false, nil
}

// UpdatePhoto patches metadata fields on an existing photo.
func (s *Store) UpdatePhoto(id string, patch map[string]any) (map[string]any, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.snap.Photos {
		if p["id"] != id {
			continue
		}
		for _, key := range []string{"place", "from", "tag", "date", "fav"} {
			if v, ok := patch[key]; ok {
				p[key] = v
			}
		}
		if err := s.savePhotosLocked(); err != nil {
			return nil, true, err
		}
		return cloneMap(p), true, nil
	}
	return nil, false, nil
}

// DeletePhoto removes a photo record by id and persists.
func (s *Store) DeletePhoto(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.snap.Photos[:0]
	found := false
	for _, p := range s.snap.Photos {
		if p["id"] == id {
			found = true
			continue
		}
		out = append(out, p)
	}
	if !found {
		return false
	}
	s.snap.Photos = out
	if err := s.savePhotosLocked(); err != nil {
		log.Printf("persist: save photos after delete: %v", err)
	}
	return true
}

// inputError is a client-facing validation failure (HTTP 400).
type inputError string

func (e inputError) Error() string { return string(e) }

// BadRequest marks the error as a rejected payload rather than a persist failure.
func (e inputError) BadRequest() bool { return true }

var firmPalette = []string{
	"#111111",
	"#00A36C",
	"#1F6FEB",
	"#C2410C",
	"#7C3AED",
	"#0F766E",
	"#B45309",
	"#BE123C",
}

// PayoutList returns a shallow clone of payout records.
func (s *Store) PayoutList() []map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneMaps(s.snap.Payouts)
}

// FirmList returns a shallow clone of prop-firm records.
func (s *Store) FirmList() []map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneMaps(s.snap.PayoutFirms)
}

// AddPayout appends a payout and persists both payout lists.
func (s *Store) AddPayout(p map[string]any) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := normalizePayout(p, s.snap.PayoutFirms)
	if err != nil {
		return nil, err
	}
	s.snap.Payouts = append(s.snap.Payouts, rec)
	if err := s.savePayoutsLocked(); err != nil {
		s.snap.Payouts = s.snap.Payouts[:len(s.snap.Payouts)-1]
		return nil, err
	}
	return cloneMap(rec), nil
}

// UpdatePayout patches fields on one payout.
func (s *Store) UpdatePayout(id string, patch map[string]any) (map[string]any, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.snap.Payouts {
		if p["id"] != id {
			continue
		}
		next := cloneMap(p)
		for _, key := range []string{"date", "amount", "currency", "account", "firmId", "note"} {
			if v, ok := patch[key]; ok {
				next[key] = v
			}
		}
		rec, err := normalizePayout(next, s.snap.PayoutFirms)
		if err != nil {
			return nil, true, err
		}
		for k, v := range rec {
			p[k] = v
		}
		if err := s.savePayoutsLocked(); err != nil {
			return nil, true, err
		}
		return cloneMap(p), true, nil
	}
	return nil, false, nil
}

// DeletePayout removes a payout record by id.
func (s *Store) DeletePayout(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.snap.Payouts[:0]
	found := false
	for _, p := range s.snap.Payouts {
		if p["id"] == id {
			found = true
			continue
		}
		out = append(out, p)
	}
	if !found {
		return false
	}
	s.snap.Payouts = out
	if err := s.savePayoutsLocked(); err != nil {
		log.Printf("persist: save payouts after delete: %v", err)
	}
	return true
}

// AddFirm appends a prop firm. Name is required; color is assigned when omitted.
func (s *Store) AddFirm(f map[string]any) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := normalizeFirm(f, true)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(asString(rec["color"])) == "" {
		rec["color"] = s.nextFirmColorLocked()
	}
	s.snap.PayoutFirms = append(s.snap.PayoutFirms, rec)
	if err := s.savePayoutsLocked(); err != nil {
		s.snap.PayoutFirms = s.snap.PayoutFirms[:len(s.snap.PayoutFirms)-1]
		return nil, err
	}
	return cloneMap(rec), nil
}

// UpdateFirm patches a prop firm's name, prefixes, or color.
func (s *Store) UpdateFirm(id string, patch map[string]any) (map[string]any, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.snap.PayoutFirms {
		if f["id"] != id {
			continue
		}
		next := cloneMap(f)
		for _, key := range []string{"name", "prefixes", "color"} {
			if v, ok := patch[key]; ok {
				next[key] = v
			}
		}
		rec, err := normalizeFirm(next, false)
		if err != nil {
			return nil, true, err
		}
		for k, v := range rec {
			f[k] = v
		}
		if err := s.savePayoutsLocked(); err != nil {
			return nil, true, err
		}
		return cloneMap(f), true, nil
	}
	return nil, false, nil
}

// DeleteFirm removes a firm and clears that firm id on existing payouts.
func (s *Store) DeleteFirm(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.snap.PayoutFirms[:0]
	found := false
	for _, f := range s.snap.PayoutFirms {
		if f["id"] == id {
			found = true
			continue
		}
		out = append(out, f)
	}
	if !found {
		return false
	}
	s.snap.PayoutFirms = out
	for _, p := range s.snap.Payouts {
		if p["firmId"] == id {
			p["firmId"] = ""
		}
	}
	if err := s.savePayoutsLocked(); err != nil {
		log.Printf("persist: save payouts after firm delete: %v", err)
	}
	return true
}

func (s *Store) nextFirmColorLocked() string {
	used := map[string]bool{}
	for _, f := range s.snap.PayoutFirms {
		used[strings.ToLower(asString(f["color"]))] = true
	}
	for _, c := range firmPalette {
		if !used[strings.ToLower(c)] {
			return c
		}
	}
	if len(firmPalette) == 0 {
		return "#111111"
	}
	return firmPalette[len(s.snap.PayoutFirms)%len(firmPalette)]
}

func normalizePayout(p map[string]any, firms []map[string]any) (map[string]any, error) {
	id := strings.TrimSpace(asString(p["id"]))
	if id == "" {
		return nil, inputError("missing id")
	}
	date, ok := asDate(p["date"])
	if !ok {
		return nil, inputError("date must be YYYY-MM-DD")
	}
	amount, ok := asAmount(p["amount"])
	if !ok {
		return nil, inputError("amount must be a number")
	}
	currency := ""
	if amount != nil {
		currency = "USD"
	}
	firmID := strings.TrimSpace(asString(p["firmId"]))
	if firmID != "" && !firmExists(firms, firmID) {
		return nil, inputError("unknown firm")
	}
	note := strings.TrimSpace(asString(p["note"]))
	if len(note) > 240 {
		note = note[:240]
	}
	account := strings.TrimSpace(asString(p["account"]))
	if len(account) > 80 {
		account = account[:80]
	}
	image := strings.TrimSpace(asString(p["image"]))
	if image == "" {
		image = id
	}
	return map[string]any{
		"id":       id,
		"date":     date,
		"amount":   amount,
		"currency": currency,
		"account":  account,
		"firmId":   firmID,
		"note":     note,
		"image":    image,
	}, nil
}

func normalizeFirm(f map[string]any, requireID bool) (map[string]any, error) {
	id := strings.TrimSpace(asString(f["id"]))
	if requireID && id == "" {
		return nil, inputError("missing id")
	}
	name := strings.TrimSpace(asString(f["name"]))
	if name == "" {
		return nil, inputError("firm name is required")
	}
	if len(name) > 48 {
		name = name[:48]
	}
	prefixes := normalizePrefixes(f["prefixes"])
	color := strings.TrimSpace(asString(f["color"]))
	if len(color) > 32 {
		color = color[:32]
	}
	return map[string]any{
		"id":       id,
		"name":     name,
		"prefixes": prefixes,
		"color":    color,
	}, nil
}

func normalizePrefixes(v any) []string {
	var parts []string
	switch t := v.(type) {
	case string:
		parts = strings.Split(t, ",")
	case []string:
		for _, s := range t {
			parts = append(parts, strings.Split(s, ",")...)
		}
	case []any:
		for _, item := range t {
			if s, ok := item.(string); ok {
				parts = append(parts, strings.Split(s, ",")...)
			}
		}
	}
	out := []string{}
	seen := map[string]bool{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if len(p) > 24 {
			p = p[:24]
		}
		key := strings.ToUpper(p)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
		if len(out) == 8 {
			break
		}
	}
	return out
}

func firmExists(firms []map[string]any, id string) bool {
	for _, f := range firms {
		if asString(f["id"]) == id {
			return true
		}
	}
	return false
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asDate(v any) (string, bool) {
	if v == nil {
		return "", true
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", true
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return "", false
	}
	return s, true
}

func asAmount(v any) (any, bool) {
	if v == nil {
		return nil, true
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return nil, false
		}
		return f, true
	default:
		return nil, false
	}
}

func cloneMaps(list []map[string]any) []map[string]any {
	out := make([]map[string]any, len(list))
	for i, m := range list {
		out[i] = cloneMap(m)
	}
	return out
}

func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
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

// Apply runs a named action and returns the new snapshot.
func (s *Store) Apply(name string, payload map[string]any) (Snapshot, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ensureChoreDayLocked() {
		// Persist roll even if the action itself is unrelated.
		if err := s.saveMutableLocked(); err != nil {
			log.Printf("persist: save after chore day roll: %v", err)
		}
	}

	switch name {
	case "setChore":
		s.setChore(payload)
	case "addChore":
		s.addChore(payload)
	case "delayChore":
		s.delayChore(payload)
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
	case "addEvent":
		if s.snap.CalendarLive {
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
		id, _ := payload["id"].(string)
		if id == "" {
			id, _ = payload["seed"].(string)
		}
		for _, p := range s.snap.Photos {
			if p["id"] == id {
				fav, _ := p["fav"].(bool)
				p["fav"] = !fav
				break
			}
		}
	default:
		// unknown — still return snapshot
	}
	s.persistAfterMutation(name)
	return s.cloneLocked(), "", nil
}

func (s *Store) setChore(p map[string]any) {
	id, _ := p["id"].(string)
	done, _ := p["done"].(bool)
	doneBy, _ := p["doneBy"].(string)
	today := todayKey()
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
			delete(c, "doneOn")
			if earner != "" && stars > 0 {
				s.snap.Stars[earner] -= stars
			}
		} else if !wasDone && done {
			c["done"] = true
			c["doneOn"] = today
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
		s.refreshHitsLocked(who, today)
		s.recomputeWeekLocked(today)
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
	today := todayKey()
	if day == "" {
		day = today
	} else if _, err := time.Parse("2006-01-02", day); err != nil {
		day = today
	}
	recurring, _ := p["recurring"].(bool)
	if max := maxChoreID(s.snap.Chores); s.nextCID < max {
		s.nextCID = max
	}
	s.nextCID++
	chore := map[string]any{
		"id":        "c" + itoa(s.nextCID),
		"title":     title,
		"who":       who,
		"r":         r,
		"emoji":     emoji,
		"stars":     stars,
		"day":       day,
		"done":      false,
		"recurring": recurring,
	}
	s.snap.Chores = append(s.snap.Chores, chore)
	s.refreshHitsLocked(who, today)
	s.recomputeWeekLocked(today)
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

// HouseholdPath resolves household.json next to the binary or from known relative paths.
func HouseholdPath() string {
	candidates := []string{
		"internal/hq/household.json",
		filepath.Join("..", "..", "internal", "hq", "household.json"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "internal/hq/household.json"
}
