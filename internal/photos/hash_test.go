package photos

import (
	"reflect"
	"testing"
)

func TestDuplicateIDsKeepsOldest(t *testing.T) {
	photos := []map[string]any{
		{"id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "hash": "abc", "fav": false},
		{"id": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "hash": "abc", "fav": false},
		{"id": "cccccccccccccccccccccccccccccccc", "hash": "def", "fav": false},
	}
	got := DuplicateIDs(photos)
	want := []string{"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestDuplicateIDsPrefersFavourite(t *testing.T) {
	photos := []map[string]any{
		{"id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "hash": "abc", "fav": false},
		{"id": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "hash": "abc", "fav": true},
		{"id": "cccccccccccccccccccccccccccccccc", "hash": "abc", "fav": false},
	}
	got := DuplicateIDs(photos)
	want := []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"cccccccccccccccccccccccccccccccc",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestDuplicateIDsIgnoresMissingHash(t *testing.T) {
	photos := []map[string]any{
		{"id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "fav": false},
		{"id": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "fav": false},
		{"id": "cccccccccccccccccccccccccccccccc", "hash": "solo", "fav": false},
	}
	if got := DuplicateIDs(photos); len(got) != 0 {
		t.Fatalf("expected none, got %v", got)
	}
}

func TestHashJPEGStable(t *testing.T) {
	a := HashJPEG([]byte("hello"))
	b := HashJPEG([]byte("hello"))
	c := HashJPEG([]byte("world"))
	if a != b {
		t.Fatal("same input different hash")
	}
	if a == c {
		t.Fatal("different input same hash")
	}
	if len(a) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(a))
	}
}
