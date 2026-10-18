package hq

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func withFixedNow(t *testing.T, day string) {
	t.Helper()
	parsed, err := time.ParseInLocation("2006-01-02", day, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	prev := nowFunc
	nowFunc = func() time.Time { return parsed.Add(15 * time.Hour) }
	t.Cleanup(func() { nowFunc = prev })
}

func loadStore(t *testing.T) *Store {
	t.Helper()
	store, err := Load(filepath.Join("household.json"))
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func weekFor(t *testing.T, snap Snapshot, who string) []int {
	t.Helper()
	var week map[string][]int
	if err := json.Unmarshal(snap.Week, &week); err != nil {
		t.Fatal(err)
	}
	return week[who]
}

func TestRecurringResetsNextDay(t *testing.T) {
	withFixedNow(t, "2026-10-12") // Monday
	store := loadStore(t)

	_, _, err := store.Apply("addChore", map[string]any{
		"title": "Brush teeth", "who": "mayura", "r": "morning", "recurring": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	id := store.Snapshot().Chores[0]["id"].(string)
	_, _, err = store.Apply("setChore", map[string]any{"id": id, "done": true})
	if err != nil {
		t.Fatal(err)
	}
	c := store.Snapshot().Chores[0]
	if c["done"] != true || c["doneOn"] != "2026-10-12" {
		t.Fatalf("expected done today, got %+v", c)
	}

	withFixedNow(t, "2026-10-13")
	store.choreDay = "2026-10-12" // force day roll on next Snapshot
	c = store.Snapshot().Chores[0]
	if c["done"] != false {
		t.Fatalf("expected recurring reset, got %+v", c)
	}
	if _, ok := c["doneOn"]; ok {
		t.Fatalf("expected doneOn cleared, got %+v", c)
	}
}

func TestWeekHitAllDone(t *testing.T) {
	withFixedNow(t, "2026-10-14") // Wednesday
	store := loadStore(t)

	_, _, _ = store.Apply("addChore", map[string]any{
		"title": "A", "who": "mayura", "r": "morning", "recurring": true,
	})
	_, _, _ = store.Apply("addChore", map[string]any{
		"title": "B", "who": "mayura", "r": "morning", "recurring": true,
	})
	snap := store.Snapshot()
	ids := []string{snap.Chores[0]["id"].(string), snap.Chores[1]["id"].(string)}

	_, _, _ = store.Apply("setChore", map[string]any{"id": ids[0], "done": true})
	w := weekFor(t, store.Snapshot(), "mayura")
	if w[2] != 0 { // Wed index 2
		t.Fatalf("partial complete should not hit: %v", w)
	}

	_, _, _ = store.Apply("setChore", map[string]any{"id": ids[1], "done": true})
	w = weekFor(t, store.Snapshot(), "mayura")
	if w[2] != 1 {
		t.Fatalf("all done should hit Wed: %v", w)
	}

	_, _, _ = store.Apply("setChore", map[string]any{"id": ids[1], "done": false})
	w = weekFor(t, store.Snapshot(), "mayura")
	if w[2] != 0 {
		t.Fatalf("undo should clear hit: %v", w)
	}
}

func TestDelayChoreMovesDay(t *testing.T) {
	withFixedNow(t, "2026-10-14")
	store := loadStore(t)
	_, _, _ = store.Apply("addChore", map[string]any{
		"title": "Later", "who": "denise", "r": "after", "day": "2026-10-14",
	})
	id := store.Snapshot().Chores[0]["id"].(string)
	_, _, _ = store.Apply("delayChore", map[string]any{"id": id, "days": 2})
	c := store.Snapshot().Chores[0]
	if c["day"] != "2026-10-16" {
		t.Fatalf("day: got %v want 2026-10-16", c["day"])
	}
}

func TestDelayClearsTodayHitBlocker(t *testing.T) {
	withFixedNow(t, "2026-10-14")
	store := loadStore(t)
	_, _, _ = store.Apply("addChore", map[string]any{
		"title": "Keep", "who": "denise", "r": "morning", "recurring": true,
	})
	_, _, _ = store.Apply("addChore", map[string]any{
		"title": "Snooze", "who": "denise", "r": "morning",
	})
	snap := store.Snapshot()
	keep := snap.Chores[0]["id"].(string)
	snooze := snap.Chores[1]["id"].(string)

	_, _, _ = store.Apply("setChore", map[string]any{"id": keep, "done": true})
	w := weekFor(t, store.Snapshot(), "denise")
	if w[2] != 0 {
		t.Fatalf("open snooze should block hit: %v", w)
	}

	_, _, _ = store.Apply("delayChore", map[string]any{"id": snooze, "days": 1})
	w = weekFor(t, store.Snapshot(), "denise")
	if w[2] != 1 {
		t.Fatalf("after delay only keep remains done → hit: %v", w)
	}
}

func TestWeekMondayIndex(t *testing.T) {
	withFixedNow(t, "2026-10-12") // Monday
	store := loadStore(t)
	_, _, _ = store.Apply("addChore", map[string]any{
		"title": "Mon", "who": "mayura", "r": "morning", "recurring": true,
	})
	id := store.Snapshot().Chores[0]["id"].(string)
	_, _, _ = store.Apply("setChore", map[string]any{"id": id, "done": true})
	w := weekFor(t, store.Snapshot(), "mayura")
	if w[0] != 1 || w[1] != 0 {
		t.Fatalf("Monday should be index 0: %v", w)
	}
}

func TestOneOffDoesNotBlockLaterDays(t *testing.T) {
	withFixedNow(t, "2026-10-12")
	store := loadStore(t)
	_, _, _ = store.Apply("addChore", map[string]any{
		"title": "Once", "who": "mayura", "r": "morning", "day": "2026-10-12",
	})
	_, _, _ = store.Apply("addChore", map[string]any{
		"title": "Daily", "who": "mayura", "r": "morning", "recurring": true,
	})
	snap := store.Snapshot()
	once := snap.Chores[0]["id"].(string)
	daily := snap.Chores[1]["id"].(string)
	_, _, _ = store.Apply("setChore", map[string]any{"id": once, "done": true})
	_, _, _ = store.Apply("setChore", map[string]any{"id": daily, "done": true})

	withFixedNow(t, "2026-10-13")
	store.choreDay = "2026-10-12"
	_ = store.Snapshot() // rolls daily

	_, _, _ = store.Apply("setChore", map[string]any{"id": daily, "done": true})
	w := weekFor(t, store.Snapshot(), "mayura")
	if w[0] != 1 || w[1] != 1 {
		t.Fatalf("finished one-off should not block Tue: %v", w)
	}
}
