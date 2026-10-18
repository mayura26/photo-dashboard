// gcal-auth performs a one-time OAuth consent and prints GOOGLE_REFRESH_TOKEN.
//
// Usage:
//
//	GOOGLE_CLIENT_ID=... GOOGLE_CLIENT_SECRET=... go run ./cmd/gcal-auth
//
// Create an OAuth Desktop client in Google Cloud Console, enable Calendar API,
// then paste the printed refresh token into .env.
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

func main() {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		log.Fatal("set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	redirectURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{calendar.CalendarReadonlyScope},
		Endpoint:     google.Endpoint,
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			errCh <- fmt.Errorf("oauth error: %s", errMsg)
			http.Error(w, "Authorization failed: "+errMsg, http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, "Authorized. You can close this tab and return to the terminal.")
		codeCh <- code
	})
	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	authURL := cfg.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Println("Open this URL in a browser and sign in with the Google account that can read your calendars:")
	fmt.Println()
	fmt.Println(authURL)
	fmt.Println()

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		log.Fatal(err)
	case <-time.After(5 * time.Minute):
		log.Fatal("timed out waiting for OAuth callback")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)

	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		log.Fatal("token exchange: ", err)
	}
	if tok.RefreshToken == "" {
		log.Fatal("no refresh token returned — revoke prior access at https://myaccount.google.com/permissions and try again")
	}

	fmt.Println("Add these to your .env (or Coolify env):")
	fmt.Println()
	fmt.Printf("GOOGLE_CLIENT_ID=%s\n", clientID)
	fmt.Printf("GOOGLE_CLIENT_SECRET=%s\n", clientSecret)
	fmt.Printf("GOOGLE_REFRESH_TOKEN=%s\n", tok.RefreshToken)
	fmt.Println()
	fmt.Println("Then set GCAL_ALEX / GCAL_JORDAN / GCAL_FAMILY to calendar IDs from Google Calendar → Settings → Integrate calendar.")
}
