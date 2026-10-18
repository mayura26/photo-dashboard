package persist

import (
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hq.db")

	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, ok, err := db.LoadMutable()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected empty db")
	}

	in := Mutable{
		Groceries: []map[string]any{
			{"id": "g1", "name": "Milk", "cat": "Dairy", "qty": 2.0, "store": "grocer", "done": false, "note": ""},
		},
		Chores: []map[string]any{
			{"id": "c1", "title": "Dishes", "done": true, "stars": 1.0, "who": "any"},
		},
		Stars:   map[string]float64{"mayura": 3},
		NextGID: 142,
	}
	if err := db.SaveMutable(in); err != nil {
		t.Fatal(err)
	}

	out, ok, err := db.LoadMutable()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected data after save")
	}
	if out.NextGID != 142 {
		t.Fatalf("nextGID: got %d want 142", out.NextGID)
	}
	if len(out.Groceries) != 1 || out.Groceries[0]["name"] != "Milk" {
		t.Fatalf("groceries: %+v", out.Groceries)
	}
	if len(out.Chores) != 1 || out.Chores[0]["title"] != "Dishes" {
		t.Fatalf("chores: %+v", out.Chores)
	}
	if out.Stars["mayura"] != 3 {
		t.Fatalf("stars: %+v", out.Stars)
	}

	// Re-open file to confirm durability.
	_ = db.Close()
	db2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	out2, ok, err := db2.LoadMutable()
	if err != nil || !ok {
		t.Fatalf("reload: ok=%v err=%v", ok, err)
	}
	if out2.NextGID != 142 || out2.Groceries[0]["name"] != "Milk" {
		t.Fatalf("reload mismatch: %+v", out2)
	}
}

func TestSaveNilSlices(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "hq.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.SaveMutable(Mutable{}); err != nil {
		t.Fatal(err)
	}
	out, ok, err := db.LoadMutable()
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if out.Groceries == nil || out.Chores == nil || out.Stars == nil {
		t.Fatalf("nil collections: %+v", out)
	}
	if out.NextGID != 100 {
		t.Fatalf("default nextGID: %d", out.NextGID)
	}
}
