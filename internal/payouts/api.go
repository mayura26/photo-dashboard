package payouts

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

// Library is the payout metadata store used by HTTP handlers.
type Library interface {
	AddPayout(p map[string]any) (map[string]any, error)
	UpdatePayout(id string, patch map[string]any) (map[string]any, bool, error)
	DeletePayout(id string) bool
	PayoutList() []map[string]any
	FirmList() []map[string]any
	AddFirm(f map[string]any) (map[string]any, error)
	UpdateFirm(id string, patch map[string]any) (map[string]any, bool, error)
	DeleteFirm(id string) bool
}

// Broadcaster pushes a fresh snapshot after mutations.
type Broadcaster interface {
	BroadcastSnapshot()
}

// API serves payout upload, edit, firm, and media routes.
type API struct {
	Dir     Dir
	Lib     Library
	Hub     Broadcaster
	Extract func(jpeg []byte, firms []Firm) Extracted
}

// HandleCollection serves POST /api/payouts (multipart upload).
func (a *API) HandleCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.handleUpload(w, r)
}

// HandleItem serves PATCH/DELETE /api/payouts/{id}.
func (a *API) HandleItem(w http.ResponseWriter, r *http.Request) {
	id := itemID(r.URL.Path, "/api/payouts/")
	if !ValidID(id) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodPatch:
		a.handlePatch(w, r, id)
	case http.MethodDelete:
		a.handleDelete(w, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleFirms serves POST /api/payout-firms.
func (a *API) HandleFirms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	patch, err := readJSON(r, 1<<16)
	if err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	id, err := NewID()
	if err != nil {
		http.Error(w, "id", http.StatusInternalServerError)
		return
	}
	rec := map[string]any{
		"id":       id,
		"name":     patch["name"],
		"prefixes": patch["prefixes"],
	}
	saved, err := a.Lib.AddFirm(rec)
	if err != nil {
		a.writeStoreErr(w, err)
		return
	}
	a.Hub.BroadcastSnapshot()
	writeJSON(w, http.StatusCreated, saved)
}

// HandleFirm serves PATCH/DELETE /api/payout-firms/{id}.
func (a *API) HandleFirm(w http.ResponseWriter, r *http.Request) {
	id := itemID(r.URL.Path, "/api/payout-firms/")
	if !ValidID(id) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodPatch:
		patch, err := readJSON(r, 1<<16)
		if err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		saved, ok, err := a.Lib.UpdateFirm(id, patch)
		if err != nil {
			a.writeStoreErr(w, err)
			return
		}
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		a.Hub.BroadcastSnapshot()
		writeJSON(w, http.StatusOK, saved)
	case http.MethodDelete:
		if !a.Lib.DeleteFirm(id) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		a.Hub.BroadcastSnapshot()
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleMedia serves GET /media/payouts/{id}.jpg.
func (a *API) HandleMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.ToLower(strings.TrimPrefix(r.URL.Path, "/media/payouts/"))
	id := strings.TrimSuffix(name, Ext)
	if !ValidID(id) || name != id+Ext {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	f, err := os.Open(a.Dir.FilePath(id))
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
	_ = hdr

	jpeg, err := EncodeJPEG(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id, err := NewID()
	if err != nil {
		http.Error(w, "id", http.StatusInternalServerError)
		return
	}
	if err := a.Dir.WriteAtomic(id, jpeg); err != nil {
		log.Printf("payouts: write %s: %v", id, err)
		http.Error(w, "write failed", http.StatusInternalServerError)
		return
	}

	firms := FirmsFromMaps(a.Lib.FirmList())
	ex := a.extract(jpeg, firms)
	firmID := ""
	if ex.OK {
		firmID = AssignFirm(ex.Account, ex.FirmID, firms)
	}
	var amount any
	currency := ""
	if ex.Amount != nil {
		amount = *ex.Amount
		currency = "USD"
	}
	rec := map[string]any{
		"id":       id,
		"date":     "",
		"amount":   amount,
		"currency": currency,
		"account":  "",
		"firmId":   firmID,
		"note":     "",
		"image":    id,
	}
	if ex.OK {
		rec["date"] = ex.Date
		rec["account"] = ex.Account
	}
	saved, err := a.Lib.AddPayout(rec)
	if err != nil {
		_ = a.Dir.Remove(id)
		log.Printf("payouts: meta %s: %v", id, err)
		a.writeStoreErr(w, err)
		return
	}
	a.Hub.BroadcastSnapshot()
	out := cloneRec(saved)
	out["extracted"] = ex.OK && (ex.Amount != nil || ex.Account != "" || ex.Date != "")
	writeJSON(w, http.StatusCreated, out)
}

func (a *API) handlePatch(w http.ResponseWriter, r *http.Request, id string) {
	patch, err := readJSON(r, 1<<16)
	if err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if guess, _ := patch["guessFirm"].(bool); guess {
		account, _ := patch["account"].(string)
		if _, ok := patch["account"]; !ok {
			account = payoutAccount(a.Lib.PayoutList(), id)
		}
		patch["firmId"] = MatchFirm(account, FirmsFromMaps(a.Lib.FirmList()))
	}
	delete(patch, "guessFirm")
	delete(patch, "extracted")
	saved, ok, err := a.Lib.UpdatePayout(id, patch)
	if err != nil {
		a.writeStoreErr(w, err)
		return
	}
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	a.Hub.BroadcastSnapshot()
	writeJSON(w, http.StatusOK, saved)
}

func (a *API) handleDelete(w http.ResponseWriter, id string) {
	if !a.Lib.DeletePayout(id) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := a.Dir.Remove(id); err != nil {
		log.Printf("payouts: remove file %s: %v", id, err)
	}
	a.Hub.BroadcastSnapshot()
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) extract(jpeg []byte, firms []Firm) Extracted {
	if a.Extract != nil {
		return a.Extract(jpeg, firms)
	}
	return Extract(jpeg, firms)
}

func (a *API) writeStoreErr(w http.ResponseWriter, err error) {
	var br interface{ BadRequest() bool }
	if errors.As(err, &br) && br.BadRequest() {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Error(w, "persist failed", http.StatusInternalServerError)
}

func payoutAccount(list []map[string]any, id string) string {
	for _, p := range list {
		if p["id"] == id {
			s, _ := p["account"].(string)
			return s
		}
	}
	return ""
}

func itemID(path, prefix string) string {
	id := strings.TrimPrefix(path, prefix)
	id = strings.Trim(id, "/")
	if i := strings.IndexByte(id, '/'); i >= 0 {
		id = id[:i]
	}
	return strings.ToLower(id)
}

func readJSON(r *http.Request, limit int64) (map[string]any, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, limit))
	if err != nil {
		return nil, err
	}
	var patch map[string]any
	if err := json.Unmarshal(body, &patch); err != nil {
		return nil, err
	}
	if patch == nil {
		patch = map[string]any{}
	}
	return patch, nil
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
