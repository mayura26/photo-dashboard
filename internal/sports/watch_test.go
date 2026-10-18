package sports

import (
	"testing"
)

func TestParseWatchlist(t *testing.T) {
	items, err := ParseWatchlist("team:New Zealand Warriors,league:Rugby League World Cup")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Kind != KindTeam || items[0].Value != "New Zealand Warriors" {
		t.Fatalf("item0 = %+v", items[0])
	}
	if items[1].Kind != KindLeague || items[1].Value != "Rugby League World Cup" {
		t.Fatalf("item1 = %+v", items[1])
	}
}

func TestParseWatchlistCompetitionAlias(t *testing.T) {
	items, err := ParseWatchlist("competition:5806")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Kind != KindLeague || items[0].Value != "5806" {
		t.Fatalf("got %+v", items)
	}
}

func TestParseWatchlistErrors(t *testing.T) {
	for _, raw := range []string{"", "  ", "Warriors", "foo:bar", "team:", ",,"} {
		if _, err := ParseWatchlist(raw); err == nil {
			t.Fatalf("ParseWatchlist(%q) expected error", raw)
		}
	}
}

func TestKnownLeagueIDs(t *testing.T) {
	if knownLeagueIDs["rugby league world cup"] != "5806" {
		t.Fatal("RLWC id")
	}
	if knownLeagueIDs["nrl"] != "4416" {
		t.Fatal("NRL id")
	}
}
