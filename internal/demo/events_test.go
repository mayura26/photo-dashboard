package demo

import (
	"path/filepath"
	"testing"
)

func TestSetSourceEventsMerge(t *testing.T) {
	store, err := Load(filepath.Join("seed.json"))
	if err != nil {
		t.Fatal(err)
	}
	seedCount := len(store.Snapshot().Events)
	if seedCount == 0 {
		t.Fatal("expected seed events")
	}

	store.SetSourceEvents("gcal", []map[string]any{
		{"id": "mayura:g1", "title": "GCal", "people": []string{"mayura"}, "source": "personal"},
	})
	snap := store.Snapshot()
	if !snap.CalendarLive {
		t.Fatal("expected calendarLive")
	}
	if len(snap.Events) != 1 || snap.Events[0]["id"] != "mayura:g1" {
		t.Fatalf("expected seed replaced by gcal only, got %#v", snap.Events)
	}

	store.SetSourceEvents("pmday", []map[string]any{
		{"id": "pmday:t1", "title": "Task", "people": []string{"mayura"}, "source": "pmday"},
	})
	snap = store.Snapshot()
	if len(snap.Events) != 2 {
		t.Fatalf("expected 2 merged events, got %d", len(snap.Events))
	}
	ids := map[string]bool{}
	for _, e := range snap.Events {
		id, _ := e["id"].(string)
		ids[id] = true
	}
	if !ids["mayura:g1"] || !ids["pmday:t1"] {
		t.Fatalf("missing merged ids: %#v", snap.Events)
	}

	store.SetSourceEvents("gcal", []map[string]any{
		{"id": "mayura:g2", "title": "Updated", "people": []string{"mayura"}, "source": "personal"},
	})
	snap = store.Snapshot()
	if len(snap.Events) != 2 {
		t.Fatalf("expected pmday retained after gcal replace, got %d", len(snap.Events))
	}
	ids = map[string]bool{}
	for _, e := range snap.Events {
		id, _ := e["id"].(string)
		ids[id] = true
	}
	if ids["mayura:g1"] || !ids["mayura:g2"] || !ids["pmday:t1"] {
		t.Fatalf("unexpected merge after gcal update: %#v", snap.Events)
	}
}

func TestSetCalendarPeople(t *testing.T) {
	store, err := Load(filepath.Join("seed.json"))
	if err != nil {
		t.Fatal(err)
	}
	store.SetCalendarPeople([]string{"mayura"})
	snap := store.Snapshot()
	if len(snap.CalendarPeople) != 1 || snap.CalendarPeople[0] != "mayura" {
		t.Fatalf("got %#v", snap.CalendarPeople)
	}
}
