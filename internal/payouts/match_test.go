package payouts

import "testing"

func TestMatchFirmLongestPrefix(t *testing.T) {
	firms := []Firm{
		{ID: "ft", Name: "Other", Prefixes: []string{"FT"}},
		{ID: "ftmo", Name: "FTMO", Prefixes: []string{"ftmo"}},
	}
	if got := MatchFirm("FTMO-10482", firms); got != "ftmo" {
		t.Fatalf("longest: got %q", got)
	}
	if got := MatchFirm("ft-999", firms); got != "ft" {
		t.Fatalf("short: got %q", got)
	}
	if got := MatchFirm("  Ftmo-1 ", firms); got != "ftmo" {
		t.Fatalf("case: got %q", got)
	}
	if got := MatchFirm("apex-1", firms); got != "" {
		t.Fatalf("unknown: got %q", got)
	}
	if got := MatchFirm("", firms); got != "" {
		t.Fatalf("empty account: got %q", got)
	}
	blank := []Firm{{ID: "any", Name: "Any", Prefixes: []string{"", "  "}}}
	if got := MatchFirm("ANY-1", blank); got != "" {
		t.Fatalf("empty prefix: got %q", got)
	}
}

func TestAssignFirmPrefersPrefix(t *testing.T) {
	firms := []Firm{
		{ID: "ftmo", Name: "FTMO", Prefixes: []string{"FTMO"}},
		{ID: "mff", Name: "MFF", Prefixes: []string{"MFF"}},
	}
	if got := AssignFirm("FTMO-1", "mff", firms); got != "ftmo" {
		t.Fatalf("prefix over model: got %q", got)
	}
	if got := AssignFirm("mystery", "mff", firms); got != "mff" {
		t.Fatalf("model id: got %q", got)
	}
	if got := AssignFirm("mystery", "MFF", firms); got != "mff" {
		t.Fatalf("model name: got %q", got)
	}
	if got := AssignFirm("mystery", "nope", firms); got != "" {
		t.Fatalf("unknown model: got %q", got)
	}
}
