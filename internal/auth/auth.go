// Package auth provides shared-password session cookies for the dashboard.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	cookieName   = "hq_session"
	sessionTTL   = 365 * 24 * time.Hour
	maxBodyBytes = 1 << 12 // 4 KiB
)

// Auth gates the dashboard when HQ_PASSWORD is set.
type Auth struct {
	password string
	secret   []byte
}

// FromEnv builds Auth from HQ_PASSWORD / HQ_AUTH_SECRET.
// Returns nil when HQ_PASSWORD is empty (auth disabled).
func FromEnv() *Auth {
	pw := strings.TrimSpace(os.Getenv("HQ_PASSWORD"))
	if pw == "" {
		return nil
	}
	secret := strings.TrimSpace(os.Getenv("HQ_AUTH_SECRET"))
	if secret == "" {
		// Derive a stable key from the password so rotation invalidates cookies.
		sum := sha256.Sum256([]byte("photo-dashboard/hq-session|" + pw))
		return &Auth{password: pw, secret: sum[:]}
	}
	sum := sha256.Sum256([]byte(secret))
	return &Auth{password: pw, secret: sum[:]}
}

// Enabled reports whether password auth is active.
func (a *Auth) Enabled() bool { return a != nil && a.password != "" }

// Valid reports whether r carries a valid session cookie.
func (a *Auth) Valid(r *http.Request) bool {
	if !a.Enabled() {
		return true
	}
	c, err := r.Cookie(cookieName)
	if err != nil || c.Value == "" {
		return false
	}
	return a.verifyToken(c.Value)
}

// Require wraps h so unauthenticated requests get 401.
// When auth is disabled, h is returned unchanged.
func (a *Auth) Require(h http.Handler) http.Handler {
	if !a.Enabled() {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.Valid(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// RequireFunc is Require for HandlerFunc.
func (a *Auth) RequireFunc(h http.HandlerFunc) http.HandlerFunc {
	return a.Require(h).ServeHTTP
}

// HandleStatus serves GET /api/auth/status → { required, ok }.
func (a *Auth) HandleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	required := a.Enabled()
	ok := !required || a.Valid(r)
	writeJSON(w, http.StatusOK, map[string]bool{
		"required": required,
		"ok":       ok,
	})
}

// HandleLogin serves POST /api/login with JSON { "password": "..." }.
func (a *Auth) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.Enabled() {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if !hmac.Equal([]byte(req.Password), []byte(a.password)) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	exp := time.Now().Add(sessionTTL)
	token := a.signToken(exp)
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isHTTPS(r),
		Expires:  exp,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// HandleLogout serves POST /api/logout and clears the session cookie.
func (a *Auth) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isHTTPS(r),
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *Auth) signToken(exp time.Time) string {
	payload := strconv.FormatInt(exp.Unix(), 10)
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte(payload))
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." +
		base64.RawURLEncoding.EncodeToString(sig)
}

func (a *Auth) verifyToken(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write(payload)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return false
	}
	expUnix, err := strconv.ParseInt(string(payload), 10, 64)
	if err != nil {
		return false
	}
	return time.Now().Unix() < expUnix
}

func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
