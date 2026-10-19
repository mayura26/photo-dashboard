package persist

import (
	"encoding/json"
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
		Stars:    map[string]float64{"mayura": 3},
		WeekHits: map[string]map[string]bool{"mayura": {"2026-10-13": true}},
		NextGID:  142,
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
	if !out.WeekHits["mayura"]["2026-10-13"] {
		t.Fatalf("weekHits: %+v", out.WeekHits)
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
	if out.Groceries == nil || out.Chores == nil || out.Stars == nil || out.WeekHits == nil {
		t.Fatalf("nil collections: %+v", out)
	}
	if out.NextGID != 100 {
		t.Fatalf("default nextGID: %d", out.NextGID)
	}
}

func TestGarminSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hq.db")

	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, ok, err := db.LoadGarmin()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected no garmin before save")
	}

	in := json.RawMessage(`{"displayName":"Mayura","steps":1234}`)
	if err := db.SaveGarmin(in); err != nil {
		t.Fatal(err)
	}

	out, ok, err := db.LoadGarmin()
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if string(out) != string(in) {
		t.Fatalf("garmin: got %s want %s", out, in)
	}

	_ = db.Close()
	db2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	out2, ok, err := db2.LoadGarmin()
	if err != nil || !ok {
		t.Fatalf("reload: ok=%v err=%v", ok, err)
	}
	if string(out2) != string(in) {
		t.Fatalf("reload mismatch: %s", out2)
	}
}

func TestPhotosSaveLoadRoundTrip(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "hq.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, ok, err := db.LoadPhotos()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected no photos before save")
	}

	in := []map[string]any{
		{"id": "aabbccddeeff00112233445566778899", "o": "l", "date": "2024-05-19", "place": "Park", "from": "mayura", "tag": "2024-05", "fav": true},
	}
	if err := db.SavePhotos(in); err != nil {
		t.Fatal(err)
	}
	out, ok, err := db.LoadPhotos()
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if len(out) != 1 || out[0]["id"] != in[0]["id"] || out[0]["place"] != "Park" {
		t.Fatalf("photos: %+v", out)
	}
}

func TestPayoutsSaveLoadRoundTrip(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "hq.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, _, ok, err := db.LoadPayouts()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected no payouts before save")
	}

	payouts := []map[string]any{
		{"id": "abc", "date": "2026-10-12", "amount": 1520.4, "currency": "USD", "account": "FTMO-1", "firmId": "f1", "note": "", "image": "abc"},
	}
	firms := []map[string]any{
		{"id": "f1", "name": "FTMO", "prefixes": []string{"FTMO"}, "color": "#111111"},
	}
	if err := db.SavePayouts(payouts, firms); err != nil {
		t.Fatal(err)
	}
	gotP, gotF, ok, err := db.LoadPayouts()
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if len(gotP) != 1 || gotP[0]["account"] != "FTMO-1" || gotP[0]["amount"] != 1520.4 {
		t.Fatalf("payouts: %+v", gotP)
	}
	if len(gotF) != 1 || gotF[0]["name"] != "FTMO" {
		t.Fatalf("firms: %+v", gotF)
	}

	if err := db.SavePayouts(nil, nil); err != nil {
		t.Fatal(err)
	}
	gotP, gotF, ok, err = db.LoadPayouts()
	if err != nil || !ok {
		t.Fatalf("empty reload ok=%v err=%v", ok, err)
	}
	if gotP == nil || gotF == nil || len(gotP) != 0 || len(gotF) != 0 {
		t.Fatalf("expected empty slices, payouts=%v firms=%v", gotP, gotF)
	}
}

func TestSaveGarminRejectsInvalid(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "hq.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.SaveGarmin(json.RawMessage(`not-json`)); err == nil {
		t.Fatal("expected error for invalid json")
	}
	if err := db.SaveGarmin(nil); err == nil {
		t.Fatal("expected error for empty")
	}
}
