package photos

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashJPEG returns the hex SHA-256 of encoded JPEG bytes.
func HashJPEG(jpeg []byte) string {
	sum := sha256.Sum256(jpeg)
	return hex.EncodeToString(sum[:])
}

// photoHash returns the string hash field, or "" if missing/invalid.
func photoHash(p map[string]any) string {
	h, _ := p["hash"].(string)
	return h
}

// photoID returns the string id field, or "" if missing.
func photoID(p map[string]any) string {
	id, _ := p["id"].(string)
	return id
}

func photoFav(p map[string]any) bool {
	fav, _ := p["fav"].(bool)
	return fav
}

// DuplicateIDs returns ids of extras to delete. Groups by hash; keeps a
// favourited copy when present, otherwise the oldest (first) entry.
// Photos without a hash are ignored.
func DuplicateIDs(photos []map[string]any) []string {
	groups := map[string][]map[string]any{}
	order := []string{}
	for _, p := range photos {
		h := photoHash(p)
		if h == "" || photoID(p) == "" {
			continue
		}
		if _, ok := groups[h]; !ok {
			order = append(order, h)
		}
		groups[h] = append(groups[h], p)
	}
	var remove []string
	for _, h := range order {
		g := groups[h]
		if len(g) < 2 {
			continue
		}
		keep := 0
		for i, p := range g {
			if photoFav(p) {
				keep = i
				break
			}
		}
		for i, p := range g {
			if i == keep {
				continue
			}
			remove = append(remove, photoID(p))
		}
	}
	return remove
}
