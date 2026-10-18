package sports

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultAPIKey  = "123"
	defaultBaseURL = "https://www.thesportsdb.com/api/v1/json"
)

// Client talks to TheSportsDB v1 JSON API.
type Client struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(apiKey string) *Client {
	if strings.TrimSpace(apiKey) == "" {
		apiKey = defaultAPIKey
	}
	return &Client{
		APIKey:  apiKey,
		BaseURL: defaultBaseURL,
		HTTPClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

// Team is a subset of TheSportsDB team fields.
type Team struct {
	IDTeam    string `json:"idTeam"`
	StrTeam   string `json:"strTeam"`
	IDLeague  string `json:"idLeague"`
	StrLeague string `json:"strLeague"`
	StrSport  string `json:"strSport"`
}

// League is a subset of TheSportsDB league fields.
type League struct {
	IDLeague         string `json:"idLeague"`
	StrLeague        string `json:"strLeague"`
	StrLeagueAlt     string `json:"strLeagueAlternate"`
	StrSport         string `json:"strSport"`
	StrCurrentSeason string `json:"strCurrentSeason"`
}

// Event is a subset of TheSportsDB event fields.
type Event struct {
	IDEvent      string `json:"idEvent"`
	StrEvent     string `json:"strEvent"`
	IDLeague     string `json:"idLeague"`
	StrLeague    string `json:"strLeague"`
	StrSeason    string `json:"strSeason"`
	StrHomeTeam  string `json:"strHomeTeam"`
	StrAwayTeam  string `json:"strAwayTeam"`
	IDHomeTeam   string `json:"idHomeTeam"`
	IDAwayTeam   string `json:"idAwayTeam"`
	IntHomeScore string `json:"intHomeScore"`
	IntAwayScore string `json:"intAwayScore"`
	StrTimestamp string `json:"strTimestamp"`
	DateEvent    string `json:"dateEvent"`
	StrTime      string `json:"strTime"`
	StrStatus    string `json:"strStatus"`
	StrPostponed string `json:"strPostponed"`
}

func (c *Client) SearchTeam(name string) (*Team, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("empty team name")
	}
	var wrap struct {
		Teams []Team `json:"teams"`
	}
	if err := c.get("searchteams.php", url.Values{"t": {name}}, &wrap); err != nil {
		return nil, err
	}
	if len(wrap.Teams) == 0 {
		return nil, fmt.Errorf("team not found: %q", name)
	}
	// Prefer exact (case-insensitive) match, else first result.
	lower := strings.ToLower(name)
	for i := range wrap.Teams {
		t := &wrap.Teams[i]
		if strings.ToLower(t.StrTeam) == lower {
			return t, nil
		}
	}
	return &wrap.Teams[0], nil
}

func (c *Client) LookupTeam(id string) (*Team, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("empty team id")
	}
	var wrap struct {
		Teams []Team `json:"teams"`
	}
	if err := c.get("lookupteam.php", url.Values{"id": {id}}, &wrap); err != nil {
		return nil, err
	}
	if len(wrap.Teams) == 0 {
		return nil, fmt.Errorf("team id not found: %s", id)
	}
	return &wrap.Teams[0], nil
}

func (c *Client) LookupLeague(id string) (*League, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("empty league id")
	}
	var wrap struct {
		Leagues []League `json:"leagues"`
	}
	if err := c.get("lookupleague.php", url.Values{"id": {id}}, &wrap); err != nil {
		return nil, err
	}
	if len(wrap.Leagues) == 0 {
		return nil, fmt.Errorf("league id not found: %s", id)
	}
	return &wrap.Leagues[0], nil
}

// SearchLeagueByName finds a league by display name or alternate, optionally scoped by sport.
func (c *Client) SearchLeagueByName(name, sport string) (*League, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("empty league name")
	}
	q := url.Values{}
	if sport != "" {
		q.Set("s", sport)
	}
	var wrap struct {
		Countries []League `json:"countries"`
	}
	if err := c.get("search_all_leagues.php", q, &wrap); err != nil {
		return nil, err
	}
	lower := strings.ToLower(name)
	var fuzzy *League
	for i := range wrap.Countries {
		l := &wrap.Countries[i]
		if strings.ToLower(l.StrLeague) == lower {
			return l, nil
		}
		if strings.EqualFold(l.StrLeagueAlt, name) {
			return l, nil
		}
		if fuzzy == nil && (strings.Contains(strings.ToLower(l.StrLeague), lower) ||
			strings.Contains(strings.ToLower(l.StrLeagueAlt), lower)) {
			fuzzy = l
		}
	}
	if fuzzy != nil {
		return fuzzy, nil
	}
	return nil, fmt.Errorf("league not found: %q", name)
}

func (c *Client) EventsSeason(leagueID, season string) ([]Event, error) {
	leagueID = strings.TrimSpace(leagueID)
	season = strings.TrimSpace(season)
	if leagueID == "" || season == "" {
		return nil, fmt.Errorf("league id and season required")
	}
	var wrap struct {
		Events []Event `json:"events"`
	}
	if err := c.get("eventsseason.php", url.Values{"id": {leagueID}, "s": {season}}, &wrap); err != nil {
		return nil, err
	}
	return wrap.Events, nil
}

func (c *Client) get(path string, q url.Values, dest any) error {
	base := c.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	key := c.APIKey
	if key == "" {
		key = defaultAPIKey
	}
	hc := c.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}

	reqURL := strings.TrimRight(base, "/") + "/" + key + "/" + path
	if len(q) > 0 {
		reqURL += "?" + q.Encode()
	}
	resp, err := hc.Get(reqURL)
	if err != nil {
		return fmt.Errorf("thesportsdb request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("thesportsdb read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("thesportsdb status %d: %s", resp.StatusCode, truncate(body, 200))
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("thesportsdb decode: %w", err)
	}
	return nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
