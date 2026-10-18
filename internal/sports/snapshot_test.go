package sports

import (
	"encoding/json"
	"testing"
	"time"
)

func TestShortLeague(t *testing.T) {
	cases := map[string]string{
		"Australian National Rugby League": "NRL",
		"Rugby League World Cup":           "RLWC",
		"NRL":                              "NRL",
		"Custom Cup":                       "Custom Cup",
	}
	for in, want := range cases {
		if got := ShortLeague(in); got != want {
			t.Errorf("ShortLeague(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestShortTeam(t *testing.T) {
	cases := map[string]string{
		"Fiji Rugby League":             "Fiji",
		"Cook Islands Rugby League":     "Cook Islands",
		"Papua New Guinea Rugby League": "PNG",
		"Tonga Rugby League":            "Tonga",
		"Lebanon Rugby League":          "Lebanon",
		"New Zealand Warriors":          "Warriors",
		"Newcastle Knights":             "Knights",
		"Sydney Roosters":               "Roosters",
		"Manly Sea Eagles":              "Eagles",
	}
	for in, want := range cases {
		if got := ShortTeam(in); got != want {
			t.Errorf("ShortTeam(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsFinalStatus(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	past := now.Add(-2 * time.Hour)
	future := now.Add(2 * time.Hour)

	if !IsFinalStatus("FT", past, now, "10", "12") {
		t.Fatal("FT should be final")
	}
	if IsFinalStatus("NS", future, now, "", "") {
		t.Fatal("NS future should not be final")
	}
	if !IsFinalStatus("", past, now, "24", "22") {
		t.Fatal("past with scores should be final")
	}
}

func TestFormatWhen(t *testing.T) {
	loc := time.Local
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, loc) // Sunday
	fri := time.Date(2026, 10, 16, 19, 5, 0, 0, loc)  // Friday same week
	if got := FormatWhen(fri, now, true); got != "Fri" {
		t.Fatalf("final same week = %q, want Fri", got)
	}
	upcoming := time.Date(2026, 10, 25, 13, 20, 0, 0, loc)
	got := FormatWhen(upcoming, now, false)
	if got != "Sun 1:20" {
		t.Fatalf("upcoming = %q, want Sun 1:20", got)
	}
}

func TestBuildFixturesTeamAndLeague(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	watches := []Resolved{
		{Kind: KindTeam, TeamID: "135193", TeamName: "New Zealand Warriors", LeagueID: "4416", League: "Australian National Rugby League", Season: "2026"},
		{Kind: KindLeague, LeagueID: "5806", League: "Rugby League World Cup", Season: "2026"},
	}
	events := map[string][]Event{
		"4416": {
			{
				IDEvent: "1", StrLeague: "Australian National Rugby League",
				StrHomeTeam: "New Zealand Warriors", StrAwayTeam: "Newcastle Knights",
				IDHomeTeam: "135193", IDAwayTeam: "135198",
				IntHomeScore: "10", IntAwayScore: "12",
				StrTimestamp: "2026-09-20T06:05:00", StrStatus: "FT",
			},
			{
				IDEvent: "2", StrLeague: "Australian National Rugby League",
				StrHomeTeam: "Penrith Panthers", StrAwayTeam: "New Zealand Warriors",
				IDHomeTeam: "135197", IDAwayTeam: "135193",
				StrTimestamp: "2027-03-01T05:00:00", StrStatus: "NS",
			},
			{
				IDEvent: "3", StrLeague: "Australian National Rugby League",
				StrHomeTeam: "Other A", StrAwayTeam: "Other B",
				IDHomeTeam: "1", IDAwayTeam: "2",
				StrTimestamp: "2026-10-01T05:00:00", StrStatus: "FT",
				IntHomeScore: "1", IntAwayScore: "2",
			},
		},
		"5806": {
			{
				IDEvent: "10", StrLeague: "Rugby League World Cup",
				StrHomeTeam: "Fiji Rugby League", StrAwayTeam: "Cook Islands Rugby League",
				IDHomeTeam: "a", IDAwayTeam: "b",
				StrTimestamp: "2026-10-18T09:00:00", StrStatus: "NS",
			},
			{
				IDEvent: "11", StrLeague: "Rugby League World Cup",
				StrHomeTeam: "Australia Rugby League", StrAwayTeam: "New Zealand Rugby League",
				IDHomeTeam: "c", IDAwayTeam: "d",
				IntHomeScore: "24", IntAwayScore: "22",
				StrTimestamp: "2026-10-15T08:00:00", StrStatus: "FT",
			},
		},
	}

	fixtures := BuildFixtures(watches, events, now)
	if len(fixtures) == 0 {
		t.Fatal("expected fixtures")
	}
	if len(fixtures) > maxFixtures {
		t.Fatalf("got %d fixtures, max %d", len(fixtures), maxFixtures)
	}

	// First should be upcoming (Warriors away or RLWC Fiji).
	if fixtures[0].Status != "upcoming" {
		t.Fatalf("first status = %s, want upcoming", fixtures[0].Status)
	}

	var foundWarriorsFinal, foundRLWC bool
	for _, f := range fixtures {
		if f.Team == "Warriors" && f.Status == "final" && f.Opp == "Knights" {
			foundWarriorsFinal = true
			if !f.Home || len(f.Score) != 2 || f.Score[0] != 10 || f.Score[1] != 12 {
				t.Fatalf("warriors final = %+v", f)
			}
			if f.League != "NRL" {
				t.Fatalf("league = %q", f.League)
			}
		}
		if f.League == "RLWC" {
			foundRLWC = true
		}
	}
	if !foundWarriorsFinal {
		t.Fatal("missing Warriors final")
	}
	if !foundRLWC {
		t.Fatal("missing RLWC fixture")
	}
}

func TestBuildSnapshotJSONNullScore(t *testing.T) {
	raw, err := BuildSnapshotJSON([]Fixture{
		{Team: "A", League: "NRL", Opp: "B", When: "Fri", Status: "upcoming", Home: true},
		{Team: "A", League: "NRL", Opp: "C", When: "Sat", Status: "final", Score: []int{18, 12}, Home: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Fixtures []struct {
			Score json.RawMessage `json:"score"`
			Status string         `json:"status"`
		} `json:"fixtures"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if string(payload.Fixtures[0].Score) != "null" {
		t.Fatalf("upcoming score = %s, want null", payload.Fixtures[0].Score)
	}
	if string(payload.Fixtures[1].Score) != "[18,12]" {
		t.Fatalf("final score = %s", payload.Fixtures[1].Score)
	}
}

func TestEventToFixtureAwayTeam(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ev := Event{
		StrLeague: "Australian National Rugby League",
		StrHomeTeam: "Penrith Panthers", StrAwayTeam: "New Zealand Warriors",
		IDHomeTeam: "135197", IDAwayTeam: "135193",
		StrTimestamp: "2027-03-01T05:00:00", StrStatus: "NS",
	}
	rf, ok := eventToFixture(ev, "135193", "New Zealand Warriors", now)
	if !ok {
		t.Fatal("expected ok")
	}
	if rf.Home || rf.Team != "Warriors" || rf.Opp != "Panthers" {
		t.Fatalf("got %+v", rf.Fixture)
	}
}
