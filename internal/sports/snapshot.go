package sports

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxFixtures = 6

// Fixture matches the Home sports strip contract.
type Fixture struct {
	Team   string `json:"team"`
	League string `json:"league"`
	Opp    string `json:"opp"`
	When   string `json:"when"`
	Status string `json:"status"` // "final" | "upcoming"
	Score  []int  `json:"score"`  // [home, away] or null when upcoming
	Home   bool   `json:"home"`
}

type rankedFixture struct {
	Fixture
	at     time.Time
	final  bool
	source string // "team" or "league" for stable sort
}

var leagueShortNames = map[string]string{
	"australian national rugby league": "NRL",
	"rugby league world cup":           "RLWC",
}

// ShortLeague returns a compact label for the home strip.
func ShortLeague(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	if s, ok := leagueShortNames[key]; ok {
		return s
	}
	// Alternate "NRL" already short.
	if strings.EqualFold(name, "NRL") {
		return "NRL"
	}
	return strings.TrimSpace(name)
}

// Known long national / club labels → glanceable short names.
var teamShortNames = map[string]string{
	"papua new guinea rugby league": "PNG",
	"papua new guinea":              "PNG",
	"cook islands rugby league":     "Cook Islands",
	"new zealand rugby league":      "New Zealand",
	"australia rugby league":        "Australia",
	"england rugby league":          "England",
	"samoa rugby league":            "Samoa",
	"tonga rugby league":            "Tonga",
	"fiji rugby league":             "Fiji",
	"lebanon rugby league":          "Lebanon",
	"france rugby league":           "France",
	"wales rugby league":            "Wales",
	"ireland rugby league":          "Ireland",
	"scotland rugby league":         "Scotland",
	"italy rugby league":            "Italy",
	"greece rugby league":           "Greece",
	"jamaica rugby league":          "Jamaica",
	"malta rugby league":            "Malta",
	"usa rugby league":              "USA",
	"south africa rugby league":     "South Africa",
	"new zealand warriors":          "Warriors",
}

// ShortTeam returns a compact team label for the home strip.
// Strips redundant "Rugby League" / club noise so "Fiji Rugby League" → "Fiji".
func ShortTeam(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return name
	}
	key := strings.ToLower(name)
	if s, ok := teamShortNames[key]; ok {
		return s
	}

	stripped := name
	for _, suf := range []string{
		" Rugby League",
		" Rugby Union",
		" Rugby",
		" Football Club",
		" FC",
	} {
		if strings.HasSuffix(strings.ToLower(stripped), strings.ToLower(suf)) {
			stripped = strings.TrimSpace(stripped[:len(stripped)-len(suf)])
			break
		}
	}
	if s, ok := teamShortNames[strings.ToLower(stripped)]; ok {
		return s
	}

	// Club nicknames: "Sydney Roosters" → "Roosters", "Manly Sea Eagles" → "Eagles".
	// Keep geo compounds: "Cook Islands", "New Zealand" (nationals already mapped above).
	words := strings.Fields(stripped)
	if len(words) >= 3 {
		return words[len(words)-1]
	}
	if len(words) == 2 && !isGeoCompound(words[0], words[1]) {
		return words[1]
	}
	return stripped
}

func isGeoCompound(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	switch {
	case a == "new" && (b == "zealand" || b == "caledonia"):
		return true
	case a == "cook" && b == "islands":
		return true
	case a == "south" && b == "africa":
		return true
	case a == "costa" && b == "rica":
		return true
	case a == "saudi" && b == "arabia":
		return true
	default:
		return false
	}
}

// IsFinalStatus reports whether a TheSportsDB status means the match is done.
func IsFinalStatus(status string, at time.Time, now time.Time, homeScore, awayScore string) bool {
	s := strings.ToUpper(strings.TrimSpace(status))
	switch s {
	case "FT", "AET", "PEN", "FINISHED", "MATCH FINISHED":
		return true
	case "NS", "TBD", "NOT STARTED", "SCHEDULED", "POSTPONED", "CANC", "CANCELLED", "SUSP":
		return false
	}
	if homeScore != "" && awayScore != "" && !at.IsZero() && at.Before(now) {
		return true
	}
	return false
}

// FormatWhen builds a compact day/time label in loc (e.g. "Fri", "Sun 1:20").
func FormatWhen(at time.Time, now time.Time, final bool, loc *time.Location) string {
	if at.IsZero() {
		return ""
	}
	loc = ensureLoc(loc)
	local := at.In(loc)
	day := local.Weekday().String()[:3] // Mon, Tue, …
	if final {
		// Same calendar week → day only; else include date hint.
		_, weekAt := local.ISOWeek()
		_, weekNow := now.In(loc).ISOWeek()
		if local.Year() == now.In(loc).Year() && weekAt == weekNow {
			return day
		}
		return day + " " + local.Format("2 Jan")
	}
	// Upcoming: include time when not midnight-ish placeholder.
	h, m, _ := local.Clock()
	if h == 0 && m == 0 {
		return day
	}
	return day + " " + local.Format("3:04")
}

func parseEventTime(ev Event, loc *time.Location) time.Time {
	if ts := strings.TrimSpace(ev.StrTimestamp); ts != "" {
		// TheSportsDB uses "2006-01-02T15:04:05" without zone (UTC-ish).
		if t, err := time.ParseInLocation("2006-01-02T15:04:05", ts, time.UTC); err == nil {
			return t
		}
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			return t
		}
	}
	date := strings.TrimSpace(ev.DateEvent)
	if date == "" {
		return time.Time{}
	}
	tm := strings.TrimSpace(ev.StrTime)
	if tm == "" || tm == "00:00:00" {
		loc = ensureLoc(loc)
		if t, err := time.ParseInLocation("2006-01-02", date, loc); err == nil {
			return t
		}
		return time.Time{}
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", date+" "+tm, time.UTC); err == nil {
		return t
	}
	return time.Time{}
}

func parseScore(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

func eventToFixture(ev Event, followedTeamID, followedTeamName string, now time.Time, loc *time.Location) (rankedFixture, bool) {
	if strings.EqualFold(ev.StrPostponed, "yes") {
		return rankedFixture{}, false
	}
	loc = ensureLoc(loc)
	at := parseEventTime(ev, loc)
	final := IsFinalStatus(ev.StrStatus, at, now, ev.IntHomeScore, ev.IntAwayScore)

	homeID := strings.TrimSpace(ev.IDHomeTeam)
	awayID := strings.TrimSpace(ev.IDAwayTeam)
	homeName := strings.TrimSpace(ev.StrHomeTeam)
	awayName := strings.TrimSpace(ev.StrAwayTeam)
	if homeName == "" || awayName == "" {
		return rankedFixture{}, false
	}

	var team, opp string
	var home bool
	source := "league"
	if followedTeamID != "" {
		source = "team"
		switch {
		case followedTeamID == homeID || (followedTeamName != "" && strings.EqualFold(followedTeamName, homeName)):
			team, opp, home = displayTeam(followedTeamName, homeName), awayName, true
		case followedTeamID == awayID || (followedTeamName != "" && strings.EqualFold(followedTeamName, awayName)):
			team, opp, home = displayTeam(followedTeamName, awayName), homeName, false
		default:
			return rankedFixture{}, false
		}
	} else {
		team, opp, home = homeName, awayName, true
	}
	team, opp = ShortTeam(team), ShortTeam(opp)

	// Drop stale "upcoming" rows (past kickoff, no result).
	if !final && !at.IsZero() && at.Before(now.Add(-3*time.Hour)) {
		return rankedFixture{}, false
	}

	f := Fixture{
		Team:   team,
		League: ShortLeague(ev.StrLeague),
		Opp:    opp,
		When:   FormatWhen(at, now, final, loc),
		Status: "upcoming",
		Home:   home,
	}
	if final {
		f.Status = "final"
		hs, hok := parseScore(ev.IntHomeScore)
		as, aok := parseScore(ev.IntAwayScore)
		if hok && aok {
			f.Score = []int{hs, as}
		}
	}
	return rankedFixture{Fixture: f, at: at, final: final, source: source}, true
}

func displayTeam(preferred, fallback string) string {
	if strings.TrimSpace(preferred) != "" {
		return preferred
	}
	return fallback
}

// BuildFixtures selects up to maxFixtures from season events for the resolved watches.
// loc is the display timezone for when labels (typically Australia/Sydney).
func BuildFixtures(watches []Resolved, eventsByLeague map[string][]Event, now time.Time, loc *time.Location) []Fixture {
	loc = ensureLoc(loc)
	seen := map[string]bool{}
	var ranked []rankedFixture

	for _, w := range watches {
		evs := eventsByLeague[w.LeagueID]
		if w.Kind == KindTeam {
			for _, ev := range evs {
				rf, ok := eventToFixture(ev, w.TeamID, w.TeamName, now, loc)
				if !ok {
					continue
				}
				key := fixtureKey(rf)
				if seen[key] {
					continue
				}
				seen[key] = true
				ranked = append(ranked, rf)
			}
		} else {
			for _, ev := range evs {
				rf, ok := eventToFixture(ev, "", "", now, loc)
				if !ok {
					continue
				}
				key := fixtureKey(rf)
				if seen[key] {
					continue
				}
				seen[key] = true
				ranked = append(ranked, rf)
			}
		}
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		// Upcoming before finals.
		if a.final != b.final {
			return !a.final
		}
		if !a.final {
			// Soonest upcoming first.
			if a.at.IsZero() != b.at.IsZero() {
				return !a.at.IsZero()
			}
			return a.at.Before(b.at)
		}
		// Most recent final first.
		return a.at.After(b.at)
	})

	// Prefer a mix: take upcoming first, then fill with recent finals.
	out := make([]Fixture, 0, maxFixtures)
	var finals []Fixture
	for _, rf := range ranked {
		if !rf.final {
			if len(out) < maxFixtures {
				out = append(out, rf.Fixture)
			}
		} else {
			finals = append(finals, rf.Fixture)
		}
	}
	for _, f := range finals {
		if len(out) >= maxFixtures {
			break
		}
		out = append(out, f)
	}
	return out
}

func fixtureKey(rf rankedFixture) string {
	if rf.at.IsZero() {
		return rf.Team + "|" + rf.Opp + "|" + rf.League + "|" + rf.When
	}
	return rf.Team + "|" + rf.Opp + "|" + rf.League + "|" + rf.at.UTC().Format(time.RFC3339)
}

// BuildSnapshotJSON returns {"fixtures":[...]} for the dashboard.
func BuildSnapshotJSON(fixtures []Fixture) (json.RawMessage, error) {
	if fixtures == nil {
		fixtures = []Fixture{}
	}
	// Ensure score is null (omitted as empty → we need explicit null for upcoming).
	type outFix struct {
		Team   string `json:"team"`
		League string `json:"league"`
		Opp    string `json:"opp"`
		When   string `json:"when"`
		Status string `json:"status"`
		Score  any    `json:"score"`
		Home   bool   `json:"home"`
	}
	payload := struct {
		Fixtures []outFix `json:"fixtures"`
	}{Fixtures: make([]outFix, len(fixtures))}
	for i, f := range fixtures {
		var score any
		if f.Status == "final" && len(f.Score) == 2 {
			score = f.Score
		} else {
			score = nil
		}
		payload.Fixtures[i] = outFix{
			Team:   f.Team,
			League: f.League,
			Opp:    f.Opp,
			When:   f.When,
			Status: f.Status,
			Score:  score,
			Home:   f.Home,
		}
	}
	return json.Marshal(payload)
}
