package photos

import "testing"

func TestClampFits(t *testing.T) {
	all := []string{"cover", "pair", "square", "wide"}
	got := ClampFits(all, "p")
	want := []string{"pair", "square"}
	if !sameIDs(got, want) {
		t.Fatalf("portrait: got %v want %v", got, want)
	}
	got = ClampFits([]string{"cover"}, "p")
	if !sameIDs(got, want) {
		t.Fatalf("portrait cover-only: got %v want %v", got, want)
	}
	got = ClampFits(all, "l")
	want = []string{"cover", "square", "wide"}
	if !sameIDs(got, want) {
		t.Fatalf("landscape: got %v want %v", got, want)
	}
	got = ClampFits([]string{"pair"}, "l")
	want = []string{"cover", "wide"}
	if !sameIDs(got, want) {
		t.Fatalf("landscape pair-only: got %v want %v", got, want)
	}
}

func sameIDs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestParseFits(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		ok   bool
		want []string
	}{
		{name: "object", raw: `{"fits":["square","wide"]}`, ok: true, want: []string{"square", "wide"}},
		{name: "order", raw: `{"fits":["wide","cover","pair"]}`, ok: true, want: []string{"cover", "pair", "wide"}},
		{name: "fenced", raw: "```json\n{\"fits\":[\"square\",\"wide\"]}\n```", ok: true, want: []string{"square", "wide"}},
		{name: "dupes", raw: `{"fits":["wide","wide","square"]}`, ok: true, want: []string{"square", "wide"}},
		{name: "drop unknown", raw: `{"fits":["cover","nope","pair"]}`, ok: true, want: []string{"cover", "pair"}},
		{name: "empty", raw: "", ok: false},
		{name: "junk", raw: "not json", ok: false},
		{name: "empty list", raw: `{"fits":[]}`, ok: false},
		{name: "unknown only", raw: `{"fits":["banner"]}`, ok: false},
		{name: "missing", raw: `{"place":"Beach"}`, ok: false},
		{name: "wrong type", raw: `{"fits":"square"}`, ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseFits(tc.raw)
			if ok != tc.ok {
				t.Fatalf("ok=%v want %v (%v)", ok, tc.ok, got)
			}
			if !tc.ok {
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v want %v", got, tc.want)
				}
			}
		})
	}
}
