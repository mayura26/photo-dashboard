package payouts_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"photo-dashboard/internal/hq"
	"photo-dashboard/internal/payouts"
	"photo-dashboard/internal/persist"
)

type nopHub struct{}

func (nopHub) BroadcastSnapshot() {}

func testJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 30))
	img.Set(0, 0, color.RGBA{R: 20, G: 40, B: 60, A: 255})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestUploadUsesPrefixOverModel(t *testing.T) {
	dir := t.TempDir()
	store, err := hq.Load(filepath.Join("..", "hq", "household.json"))
	if err != nil {
		t.Fatal(err)
	}
	db, err := persist.Open(filepath.Join(dir, "hq.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.AttachPersist(db); err != nil {
		t.Fatal(err)
	}
	ftmo, err := store.AddFirm(map[string]any{
		"id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "name": "FTMO", "prefixes": []string{"FTMO"},
	})
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.AddFirm(map[string]any{
		"id": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "name": "Other", "prefixes": []string{"FT"},
	})
	if err != nil {
		t.Fatal(err)
	}

	amt := 250.5
	api := &payouts.API{
		Dir: payouts.Dir{Path: filepath.Join(dir, "payouts")},
		Lib: store,
		Hub: nopHub{},
		Extract: func(jpeg []byte, firms []payouts.Firm) payouts.Extracted {
			if len(jpeg) == 0 || len(firms) != 2 {
				t.Fatalf("extract saw jpeg=%d firms=%d", len(jpeg), len(firms))
			}
			return payouts.Extracted{
				Amount: &amt, Currency: "USD", Date: "2026-10-02",
				Account: "ftmo-10482", FirmID: other["id"].(string), OK: true,
			}
		},
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", "payout.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(testJPEG(t)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/payouts", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	api.HandleCollection(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	var saved map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved["firmId"] != ftmo["id"] {
		t.Fatalf("firmId: got %v want %v", saved["firmId"], ftmo["id"])
	}
	if saved["account"] != "ftmo-10482" || saved["extracted"] != true {
		t.Fatalf("saved: %+v", saved)
	}
	if saved["amount"] != amt {
		t.Fatalf("amount: %v", saved["amount"])
	}
}

func TestPatchGuessFirm(t *testing.T) {
	dir := t.TempDir()
	store, err := hq.Load(filepath.Join("..", "hq", "household.json"))
	if err != nil {
		t.Fatal(err)
	}
	db, err := persist.Open(filepath.Join(dir, "hq.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.AttachPersist(db); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddFirm(map[string]any{
		"id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "name": "FTMO", "prefixes": []string{"FTMO"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddPayout(map[string]any{
		"id": "cccccccccccccccccccccccccccccccc", "date": "2026-10-01", "amount": 10.0,
		"currency": "USD", "account": "OLD", "firmId": "", "note": "", "image": "cccccccccccccccccccccccccccccccc",
	}); err != nil {
		t.Fatal(err)
	}
	api := &payouts.API{Dir: payouts.Dir{Path: filepath.Join(dir, "shots")}, Lib: store, Hub: nopHub{}}
	req := httptest.NewRequest(http.MethodPatch, "/api/payouts/cccccccccccccccccccccccccccccccc",
		bytes.NewReader([]byte(`{"account":"FTMO-9","guessFirm":true}`)))
	rr := httptest.NewRecorder()
	api.HandleItem(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	var saved map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved["firmId"] != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("firmId: %v", saved["firmId"])
	}
}

func TestDeleteFirmClearsPayout(t *testing.T) {
	store, err := hq.Load(filepath.Join("..", "hq", "household.json"))
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
	if _, err := store.AddFirm(map[string]any{
		"id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "name": "FTMO", "prefixes": []string{"FTMO"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddPayout(map[string]any{
		"id": "dddddddddddddddddddddddddddddddd", "date": "2026-09-01", "amount": 80.0,
		"currency": "USD", "account": "FTMO-1", "firmId": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"note": "", "image": "dddddddddddddddddddddddddddddddd",
	}); err != nil {
		t.Fatal(err)
	}
	api := &payouts.API{Lib: store, Hub: nopHub{}}
	req := httptest.NewRequest(http.MethodDelete, "/api/payout-firms/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil)
	rr := httptest.NewRecorder()
	api.HandleFirm(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	list := store.PayoutList()
	if len(list) != 1 || list[0]["firmId"] != "" {
		t.Fatalf("payouts: %+v", list)
	}
	if len(store.FirmList()) != 0 {
		t.Fatalf("firms: %+v", store.FirmList())
	}
}
