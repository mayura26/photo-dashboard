package demo

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"photo-dashboard/internal/persist"
)

func TestAttachPersistBlankBootstrap(t *testing.T) {
	store, err := Load(filepath.Join("seed.json"))
	if err != nil {
		t.Fatal(err)
	}
	db, err := persist.Open(filepath.Join(t.TempDir(), "hq.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := store.AttachPersist(db); err != nil {
		t.Fatal(err)
	}
	snap := store.Snapshot()
	if len(snap.Groceries) != 0 || len(snap.Chores) != 0 || len(snap.Stars) != 0 {
		t.Fatalf("expected blank bootstrap, got groceries=%d chores=%d stars=%d",
			len(snap.Groceries), len(snap.Chores), len(snap.Stars))
	}
}

func TestAttachPersistRoundTrip(t *testing.T) {
	seed := filepath.Join("seed.json")
	store, err := Load(seed)
	if err != nil {
		t.Fatal(err)
	}

	db, err := persist.Open(filepath.Join(t.TempDir(), "hq.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := store.AttachPersist(db); err != nil {
		t.Fatal(err)
	}

	_, _, err = store.Apply("addGrocery", map[string]any{"name": "Persist Milk", "cat": "Dairy"})
	if err != nil {
		t.Fatal(err)
	}

	store2, err := Load(seed)
	if err != nil {
		t.Fatal(err)
	}
	if err := store2.AttachPersist(db); err != nil {
		t.Fatal(err)
	}
	snap := store2.Snapshot()

	found := false
	for _, g := range snap.Groceries {
		if g["name"] == "Persist Milk" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected Persist Milk after reload")
	}
	if len(snap.Chores) != 0 {
		t.Fatalf("expected no chores, got %d", len(snap.Chores))
	}
}

func TestGarminSurvivesRedeploy(t *testing.T) {
	seed := filepath.Join("seed.json")
	store, err := Load(seed)
	if err != nil {
		t.Fatal(err)
	}

	db, err := persist.Open(filepath.Join(t.TempDir(), "hq.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := store.AttachPersist(db); err != nil {
		t.Fatal(err)
	}

	live := json.RawMessage(`{"displayName":"Live Mayura","steps":9876,"bodyBattery":72}`)
	store.SetGarmin(live)

	// Simulate Go redeploy: new Store from seed, same SQLite file.
	store2, err := Load(seed)
	if err != nil {
		t.Fatal(err)
	}
	seedGarmin := string(store2.Snapshot().Garmin)
	if err := store2.AttachPersist(db); err != nil {
		t.Fatal(err)
	}
	snap := store2.Snapshot()
	if string(snap.Garmin) != string(live) {
		t.Fatalf("expected live garmin after AttachPersist, got %s (seed was %s)", snap.Garmin, seedGarmin)
	}
}
