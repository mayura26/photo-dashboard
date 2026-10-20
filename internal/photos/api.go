package photos

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

// Library is the photo metadata store used by HTTP handlers.
type Library interface {
	DefaultPhotoFrom() string
	AddPhoto(p map[string]any) (map[string]any, error)
	UpdatePhoto(id string, patch map[string]any) (map[string]any, bool, error)
	DeletePhoto(id string) bool
	PhotoList() []map[string]any
	FindPhotoByHash(hash string) (map[string]any, bool)
	SetPhotoHashes(hashes map[string]string) error
	SetPhotoFits(id string, fits []string) (map[string]any, bool, error)
}

// Broadcaster pushes a fresh snapshot after mutations.
type Broadcaster interface {
	BroadcastSnapshot()
}

// API serves upload / patch / delete / media routes.
type API struct {
	Dir Dir
	Lib Library
	Hub Broadcaster
}

// HandleCollection serves POST /api/photos (multipart upload).
func (a *API) HandleCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.handleUpload(w, r)
}

// HandlePurgeDuplicates serves POST /api/photos/purge-duplicates.
func (a *API) HandlePurgeDuplicates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	removed, err := a.purgeDuplicates()
	if err != nil {
		log.Printf("photos: purge duplicates: %v", err)
		http.Error(w, "purge failed", http.StatusInternalServerError)
		return
	}
	a.Hub.BroadcastSnapshot()
	writeJSON(w, http.StatusOK, map[string]any{"removed": removed})
}

// HandleItem serves PATCH/DELETE /api/photos/{id}.
func (a *API) HandleItem(w http.ResponseWriter, r *http.Request) {
	id := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/photos/")))
	id = strings.TrimSuffix(id, Ext)
	if !ValidID(id) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodPatch:
		a.handlePatch(w, r, id)
	case http.MethodDelete:
		a.handleDelete(w, r, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleMedia serves GET /media/photos/{id}.jpg.
func (a *API) HandleMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.ToLower(strings.TrimPrefix(r.URL.Path, "/media/photos/"))
	id := strings.TrimSuffix(name, Ext)
	if !ValidID(id) || name != id+Ext {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	path := a.Dir.FilePath(id)
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", ContentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeContent(w, r, id+Ext, stat.ModTime(), f)
}

func ignoreDuplicatesForm(r *http.Request) bool {
	v := strings.ToLower(strings.TrimSpace(r.FormValue("ignoreDuplicates")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func (a *API) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBytes+1024)
	if err := r.ParseMultipartForm(MaxBytes); err != nil {
		http.Error(w, "bad multipart: "+err.Error(), http.StatusBadRequest)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()
	filename := ""
	if hdr != nil {
		filename = hdr.Filename
	}

	enc, err := Encode(file, filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	hash := HashJPEG(enc.JPEG)
	if ignoreDuplicatesForm(r) {
		if existing, ok := a.Lib.FindPhotoByHash(hash); ok {
			out := cloneRec(existing)
			out["duplicate"] = true
			writeJSON(w, http.StatusOK, out)
			return
		}
	}

	id, err := NewID()
	if err != nil {
		http.Error(w, "id", http.StatusInternalServerError)
		return
	}
	if err := a.Dir.WriteAtomic(id, enc.JPEG); err != nil {
		log.Printf("photos: write %s: %v", id, err)
		http.Error(w, "write failed", http.StatusInternalServerError)
		return
	}

	rec := map[string]any{
		"id":    id,
		"o":     enc.Orient,
		"date":  enc.Date,
		"place": enc.Place,
		"from":  a.Lib.DefaultPhotoFrom(),
		"tag":   TagFromDate(enc.Date),
		"fav":   false,
		"hash":  hash,
	}
	saved, err := a.Lib.AddPhoto(rec)
	if err != nil {
		_ = a.Dir.Remove(id)
		log.Printf("photos: meta %s: %v", id, err)
		http.Error(w, "persist failed", http.StatusInternalServerError)
		return
	}
	a.Hub.BroadcastSnapshot()
	writeJSON(w, http.StatusCreated, saved)
	if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) != "" {
		jpeg := append([]byte(nil), enc.JPEG...)
		go a.classifyPhoto(id, jpeg)
	}
}

func (a *API) purgeDuplicates() (int, error) {
	photos := a.Lib.PhotoList()
	backfill := map[string]string{}
	for _, p := range photos {
		id := photoID(p)
		if id == "" || photoHash(p) != "" {
			continue
		}
		if !ValidID(id) {
			continue
		}
		raw, err := os.ReadFile(a.Dir.FilePath(id))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return 0, err
		}
		h := HashJPEG(raw)
		backfill[id] = h
		p["hash"] = h
	}
	if err := a.Lib.SetPhotoHashes(backfill); err != nil {
		return 0, err
	}

	toRemove := DuplicateIDs(photos)
	removed := 0
	for _, id := range toRemove {
		if !a.Lib.DeletePhoto(id) {
			continue
		}
		if err := a.Dir.Remove(id); err != nil {
			log.Printf("photos: remove file %s: %v", id, err)
		}
		removed++
	}
	return removed, nil
}

func (a *API) handlePatch(w http.ResponseWriter, r *http.Request, id string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	var patch map[string]any
	if err := json.Unmarshal(body, &patch); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	saved, ok, err := a.Lib.UpdatePhoto(id, patch)
	if err != nil {
		http.Error(w, "persist failed", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	a.Hub.BroadcastSnapshot()
	writeJSON(w, http.StatusOK, saved)
}

func (a *API) handleDelete(w http.ResponseWriter, r *http.Request, id string) {
	if !a.Lib.DeletePhoto(id) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := a.Dir.Remove(id); err != nil {
		log.Printf("photos: remove file %s: %v", id, err)
	}
	a.Hub.BroadcastSnapshot()
	w.WriteHeader(http.StatusNoContent)
}

// BackfillFits classifies stored photos that have no frame list yet.
// It runs one photo at a time and does not block startup.
func (a *API) BackfillFits() {
	if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" {
		return
	}
	go a.backfillFits()
}

func (a *API) backfillFits() {
	pending := 0
	for _, p := range a.Lib.PhotoList() {
		if photoHasFits(p) {
			continue
		}
		id := photoID(p)
		if !ValidID(id) {
			continue
		}
		raw, err := os.ReadFile(a.Dir.FilePath(id))
		if err != nil {
			if !os.IsNotExist(err) {
				log.Printf("photos: frame fit %s: %v", id, err)
			}
			continue
		}
		pending++
		a.classifyPhoto(id, raw)
	}
	if pending > 0 {
		log.Printf("photos: frame fit pass finished (%d photos)", pending)
	}
}

func (a *API) classifyPhoto(id string, jpeg []byte) {
	fits, ok := ClassifyFits(jpeg)
	if !ok {
		return
	}
	if _, found, err := a.Lib.SetPhotoFits(id, fits); err != nil {
		log.Printf("photos: frame fit %s: %v", id, err)
		return
	} else if !found {
		return
	}
	log.Printf("photos: frame fit %s: %s", id, strings.Join(fits, ","))
	if a.Hub != nil {
		a.Hub.BroadcastSnapshot()
	}
}

func cloneRec(m map[string]any) map[string]any {
	out := make(map[string]any, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
