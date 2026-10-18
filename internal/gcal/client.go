package gcal

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// CalendarRef is one Google calendar to poll and how it maps onto dashboard people.
type CalendarRef struct {
	ID       string   // Google calendar id
	Source   string   // "personal" or "family"
	People   []string // person ids for events from this calendar
}

// Client lists events from Google Calendar using a refresh token.
type Client struct {
	HTTPClient *http.Client
	svc        *calendar.Service
}

// NewClient builds an OAuth2 HTTP client and Calendar API service.
func NewClient(ctx context.Context, clientID, clientSecret, refreshToken string) (*Client, error) {
	if clientID == "" || clientSecret == "" || refreshToken == "" {
		return nil, fmt.Errorf("gcal: missing GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, or GOOGLE_REFRESH_TOKEN")
	}
	cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{calendar.CalendarReadonlyScope},
	}
	tok := &oauth2.Token{RefreshToken: refreshToken}
	hc := cfg.Client(ctx, tok)
	hc.Timeout = 30 * time.Second

	svc, err := calendar.NewService(ctx, option.WithHTTPClient(hc))
	if err != nil {
		return nil, fmt.Errorf("gcal: calendar service: %w", err)
	}
	return &Client{HTTPClient: hc, svc: svc}, nil
}

// Fetch returns normalized dashboard events for the given calendars and time window.
func (c *Client) Fetch(ctx context.Context, cals []CalendarRef, timeMin, timeMax time.Time) ([]map[string]any, error) {
	if c == nil || c.svc == nil {
		return nil, fmt.Errorf("gcal: client not initialized")
	}
	var out []map[string]any
	for _, cal := range cals {
		if cal.ID == "" {
			continue
		}
		events, err := c.listCalendar(ctx, cal, timeMin, timeMax)
		if err != nil {
			return nil, err
		}
		out = append(out, events...)
	}
	return out, nil
}

func (c *Client) listCalendar(ctx context.Context, cal CalendarRef, timeMin, timeMax time.Time) ([]map[string]any, error) {
	var pageToken string
	var items []*calendar.Event
	for {
		call := c.svc.Events.List(cal.ID).
			Context(ctx).
			TimeMin(timeMin.Format(time.RFC3339)).
			TimeMax(timeMax.Format(time.RFC3339)).
			SingleEvents(true).
			OrderBy("startTime").
			MaxResults(250)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("gcal: list %s: %w", cal.ID, err)
		}
		items = append(items, resp.Items...)
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	return NormalizeEvents(items, cal), nil
}
