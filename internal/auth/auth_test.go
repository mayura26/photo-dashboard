package auth

import (
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testAuth(password string) *Auth {
	sum := sha256.Sum256([]byte("photo-dashboard/hq-session|" + password))
	return &Auth{password: password, secret: sum[:]}
}

func TestLoginSetsValidCookie(t *testing.T) {
	a := testAuth("secret")
	body := `{"password":"secret"}`
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(body))
	rec := httptest.NewRecorder()
	a.HandleLogin(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	res := rec.Result()
	var cookie *http.Cookie
	for _, c := range res.Cookies() {
		if c.Name == cookieName {
			cookie = c
			break
		}
	}
	if cookie == nil || cookie.Value == "" {
		t.Fatal("expected hq_session cookie")
	}
	if !cookie.HttpOnly {
		t.Fatal("cookie should be HttpOnly")
	}

	check := httptest.NewRequest(http.MethodGet, "/ws", nil)
	check.AddCookie(cookie)
	if !a.Valid(check) {
		t.Fatal("cookie should validate")
	}
}

func TestLoginRejectsBadPassword(t *testing.T) {
	a := testAuth("secret")
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"password":"nope"}`))
	rec := httptest.NewRecorder()
	a.HandleLogin(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	a := testAuth("secret")
	token := a.signToken(time.Now().Add(-time.Hour))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	if a.Valid(req) {
		t.Fatal("expired token should be invalid")
	}
}

func TestRequireBlocksUnauthed(t *testing.T) {
	a := testAuth("secret")
	h := a.RequireFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestStatusWhenDisabled(t *testing.T) {
	var a *Auth
	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	rec := httptest.NewRecorder()
	a.HandleStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"required":false`) {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	a := testAuth("secret")
	req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	rec := httptest.NewRecorder()
	a.HandleLogout(rec, req)
	res := rec.Result()
	found := false
	for _, c := range res.Cookies() {
		if c.Name == cookieName && c.MaxAge < 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected cleared cookie")
	}
}
