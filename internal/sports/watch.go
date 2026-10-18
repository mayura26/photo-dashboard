package sports

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Kind is a watchlist entry type.
type Kind string

const (
	KindTeam   Kind = "team"
	KindLeague Kind = "league"
)

// WatchItem is one parsed SPORTS_WATCH entry before ID resolution.
type WatchItem struct {
	Kind  Kind
	Value string // name or numeric id
}

// Resolved is a watch item with TheSportsDB ids filled in.
type Resolved struct {
	Kind     Kind
	TeamID   string
	TeamName string
	LeagueID string
	League   string
	Season   string
}

// Known league name/alias → id (avoids flaky free-tier league search).
var knownLeagueIDs = map[string]string{
	"rugby league world cup":           "5806",
	"rlwc":                             "5806",
	"australian national rugby league": "4416",
	"nrl":                              "4416",
	"national rugby league":            "4416",
}

// ParseWatchlist parses SPORTS_WATCH: comma-separated team:X or league:X entries.
// Commas inside values are not supported; use IDs if names contain commas.
func ParseWatchlist(raw string) ([]WatchItem, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty watchlist")
	}
	parts := strings.Split(raw, ",")
	out := make([]WatchItem, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		kind, val, ok := strings.Cut(p, ":")
		if !ok {
			return nil, fmt.Errorf("watch item %q: want team:… or league:…", p)
		}
		kind = strings.ToLower(strings.TrimSpace(kind))
		val = strings.TrimSpace(val)
		if val == "" {
			return nil, fmt.Errorf("watch item %q: empty value", p)
		}
		switch kind {
		case "team":
			out = append(out, WatchItem{Kind: KindTeam, Value: val})
		case "league", "competition":
			out = append(out, WatchItem{Kind: KindLeague, Value: val})
		default:
			return nil, fmt.Errorf("watch item %q: unknown kind %q (use team or league)", p, kind)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty watchlist")
	}
	return out, nil
}

// Resolve turns watch items into IDs using the client (and known league aliases).
func Resolve(c *Client, items []WatchItem) ([]Resolved, error) {
	out := make([]Resolved, 0, len(items))
	for _, it := range items {
		switch it.Kind {
		case KindTeam:
			r, err := resolveTeam(c, it.Value)
			if err != nil {
				return nil, err
			}
			out = append(out, r)
		case KindLeague:
			r, err := resolveLeague(c, it.Value)
			if err != nil {
				return nil, err
			}
			out = append(out, r)
		default:
			return nil, fmt.Errorf("unknown kind %q", it.Kind)
		}
	}
	return out, nil
}

func resolveTeam(c *Client, value string) (Resolved, error) {
	var t *Team
	var err error
	if isNumericID(value) {
		t, err = c.LookupTeam(value)
	} else {
		t, err = c.SearchTeam(value)
	}
	if err != nil {
		return Resolved{}, err
	}
	season := ""
	leagueName := t.StrLeague
	if t.IDLeague != "" {
		if lg, lerr := c.LookupLeague(t.IDLeague); lerr == nil && lg != nil {
			season = lg.StrCurrentSeason
			if lg.StrLeague != "" {
				leagueName = lg.StrLeague
			}
		}
	}
	if season == "" {
		season = strconv.Itoa(time.Now().Year())
	}
	return Resolved{
		Kind:     KindTeam,
		TeamID:   t.IDTeam,
		TeamName: t.StrTeam,
		LeagueID: t.IDLeague,
		League:   leagueName,
		Season:   season,
	}, nil
}

func resolveLeague(c *Client, value string) (Resolved, error) {
	id := value
	if !isNumericID(value) {
		key := strings.ToLower(strings.TrimSpace(value))
		if known, ok := knownLeagueIDs[key]; ok {
			id = known
		} else {
			lg, err := c.SearchLeagueByName(value, "Rugby")
			if err != nil {
				lg, err = c.SearchLeagueByName(value, "")
			}
			if err != nil {
				return Resolved{}, err
			}
			id = lg.IDLeague
		}
	}
	lg, err := c.LookupLeague(id)
	if err != nil {
		return Resolved{}, err
	}
	season := lg.StrCurrentSeason
	if season == "" {
		season = strconv.Itoa(time.Now().Year())
	}
	return Resolved{
		Kind:     KindLeague,
		LeagueID: lg.IDLeague,
		League:   lg.StrLeague,
		Season:   season,
	}, nil
}

func isNumericID(s string) bool {
	_, err := strconv.Atoi(strings.TrimSpace(s))
	return err == nil
}
