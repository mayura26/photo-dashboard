package photos

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type memLib struct {
	mu     sync.Mutex
	photos []map[string]any
}

func (m *memLib) DefaultPhotoFrom() string { return "p1" }

func (m *memLib) AddPhoto(p map[string]any) (map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := cloneRec(p)
	m.photos = append(m.photos, cp)
	return cloneRec(cp), nil
}

func (m *memLib) UpdatePhoto(id string, patch map[string]any) (map[string]any, bool, error) {
	return nil, false, nil
}

func (m *memLib) DeletePhoto(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := m.photos[:0]
	found := false
	for _, p := range m.photos {
		if p["id"] == id {
			found = true
			continue
		}
		out = append(out, p)
	}
	m.photos = out
	return found
}

func (m *memLib) PhotoList() []map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]map[string]any, len(m.photos))
	for i, p := range m.photos {
		out[i] = cloneRec(p)
	}
	return out
}

func (m *memLib) FindPhotoByHash(hash string) (map[string]any, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.photos {
		h, _ := p["hash"].(string)
		if h == hash {
			return cloneRec(p), true
		}
	}
	return nil, false
}

func (m *memLib) SetPhotoHashes(hashes map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.photos {
		id, _ := p["id"].(string)
		if h, ok := hashes[id]; ok {
			p["hash"] = h
		}
	}
	return nil
}

type nopHub struct{}

func (nopHub) BroadcastSnapshot() {}

func testJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 30))
	for y := 0; y < 30; y++ {
		for x := 0; x < 40; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func multipartUpload(t *testing.T, jpegBytes []byte, ignoreDupes bool) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", "shot.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(jpegBytes); err != nil {
		t.Fatal(err)
	}
	if ignoreDupes {
		if err := w.WriteField("ignoreDuplicates", "1"); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/photos", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestUploadIgnoreDuplicates(t *testing.T) {
	dir := Dir{Path: t.TempDir()}
	lib := &memLib{}
	api := &API{Dir: dir, Lib: lib, Hub: nopHub{}}

	jpegBytes := testJPEG(t)

	rr := httptest.NewRecorder()
	api.HandleCollection(rr, multipartUpload(t, jpegBytes, true))
	if rr.Code != http.StatusCreated {
		t.Fatalf("first upload: %d %s", rr.Code, rr.Body.String())
	}
	var first map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	hash, _ := first["hash"].(string)
	if hash == "" {
		t.Fatal("expected hash on first upload")
	}
	id, _ := first["id"].(string)

	rr2 := httptest.NewRecorder()
	api.HandleCollection(rr2, multipartUpload(t, jpegBytes, true))
	if rr2.Code != http.StatusOK {
		t.Fatalf("second upload: %d %s", rr2.Code, rr2.Body.String())
	}
	var second map[string]any
	if err := json.Unmarshal(rr2.Body.Bytes(), &second); err != nil {
		t.Fatal(err)
	}
	if second["duplicate"] != true {
		t.Fatalf("expected duplicate flag, got %#v", second["duplicate"])
	}
	if second["id"] != id {
		t.Fatalf("expected existing id %s, got %v", id, second["id"])
	}
	if len(lib.PhotoList()) != 1 {
		t.Fatalf("expected 1 photo, got %d", len(lib.PhotoList()))
	}
	entries, err := os.ReadDir(dir.Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file on disk, got %d", len(entries))
	}
}

func TestUploadWithoutIgnoreCreatesDuplicate(t *testing.T) {
	dir := Dir{Path: t.TempDir()}
	lib := &memLib{}
	api := &API{Dir: dir, Lib: lib, Hub: nopHub{}}
	jpegBytes := testJPEG(t)

	rr := httptest.NewRecorder()
	api.HandleCollection(rr, multipartUpload(t, jpegBytes, false))
	if rr.Code != http.StatusCreated {
		t.Fatalf("first: %d %s", rr.Code, rr.Body.String())
	}
	rr2 := httptest.NewRecorder()
	api.HandleCollection(rr2, multipartUpload(t, jpegBytes, false))
	if rr2.Code != http.StatusCreated {
		t.Fatalf("second: %d %s", rr2.Code, rr2.Body.String())
	}
	if len(lib.PhotoList()) != 2 {
		t.Fatalf("expected 2 photos, got %d", len(lib.PhotoList()))
	}
}

func TestPurgeDuplicates(t *testing.T) {
	dir := Dir{Path: t.TempDir()}
	lib := &memLib{}
	api := &API{Dir: dir, Lib: lib, Hub: nopHub{}}

	jpegA := testJPEG(t)
	// Slightly different image so hashes differ.
	img := image.NewRGBA(image.Rect(0, 0, 40, 30))
	for y := 0; y < 30; y++ {
		for x := 0; x < 40; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 20, B: 30, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	jpegB := buf.Bytes()

	upload := func(data []byte) string {
		rr := httptest.NewRecorder()
		api.HandleCollection(rr, multipartUpload(t, data, false))
		if rr.Code != http.StatusCreated {
			t.Fatalf("upload: %d %s", rr.Code, rr.Body.String())
		}
		var rec map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &rec); err != nil {
			t.Fatal(err)
		}
		id, _ := rec["id"].(string)
		return id
	}

	id1 := upload(jpegA)
	id2 := upload(jpegA) // duplicate of A
	id3 := upload(jpegB)
	_ = id3

	// Mark second A as favourite so purge keeps it.
	for _, p := range lib.photos {
		if p["id"] == id2 {
			p["fav"] = true
		}
	}

	rr := httptest.NewRecorder()
	api.HandlePurgeDuplicates(rr, httptest.NewRequest(http.MethodPost, "/api/photos/purge-duplicates", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("purge: %d %s", rr.Code, rr.Body.String())
	}
	var res map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res["removed"] != float64(1) {
		t.Fatalf("removed=%v want 1", res["removed"])
	}
	list := lib.PhotoList()
	if len(list) != 2 {
		t.Fatalf("expected 2 remaining, got %d", len(list))
	}
	ids := map[string]bool{}
	for _, p := range list {
		ids[p["id"].(string)] = true
	}
	if !ids[id2] || !ids[id3] || ids[id1] {
		t.Fatalf("kept wrong set: %#v (fav=%s unique=%s deleted=%s)", ids, id2, id3, id1)
	}
	if _, err := os.Stat(filepath.Join(dir.Path, id1+Ext)); !os.IsNotExist(err) {
		t.Fatalf("deleted file still present: %v", err)
	}
}

func TestPurgeBackfillsHash(t *testing.T) {
	dir := Dir{Path: t.TempDir()}
	lib := &memLib{}
	api := &API{Dir: dir, Lib: lib, Hub: nopHub{}}

	jpegBytes := testJPEG(t)
	id1, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	id2, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	if err := dir.WriteAtomic(id1, jpegBytes); err != nil {
		t.Fatal(err)
	}
	if err := dir.WriteAtomic(id2, jpegBytes); err != nil {
		t.Fatal(err)
	}
	_, _ = lib.AddPhoto(map[string]any{"id": id1, "fav": false})
	_, _ = lib.AddPhoto(map[string]any{"id": id2, "fav": false})

	rr := httptest.NewRecorder()
	api.HandlePurgeDuplicates(rr, httptest.NewRequest(http.MethodPost, "/api/photos/purge-duplicates", nil))
	if rr.Code != http.StatusOK {
		body, _ := io.ReadAll(rr.Body)
		t.Fatalf("purge: %d %s", rr.Code, body)
	}
	var res map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&res)
	if res["removed"] != float64(1) {
		t.Fatalf("removed=%v want 1", res["removed"])
	}
	if len(lib.PhotoList()) != 1 {
		t.Fatalf("expected 1 photo, got %d", len(lib.PhotoList()))
	}
	remaining := lib.PhotoList()[0]
	if remaining["hash"] == nil || remaining["hash"] == "" {
		t.Fatal("expected backfilled hash on keeper")
	}
}
