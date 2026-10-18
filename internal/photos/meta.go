package photos

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf16"

	"github.com/rwcarlsen/goexif/exif"
)

// PlaceHints are filename + EXIF/XMP clues used when GPS is missing.
type PlaceHints struct {
	Filename    string
	Date        string
	Description string
	Comment     string
	Title       string
	Keywords    []string
	Subject     string
	Artist      string
	Software    string
	Make        string
	Model       string
	XMPLocation string
	XMPCity     string
	XMPCountry  string
	XMPExtra    []string
}

func (h PlaceHints) useful() bool {
	if strings.TrimSpace(h.Description) != "" ||
		strings.TrimSpace(h.Comment) != "" ||
		strings.TrimSpace(h.Title) != "" ||
		strings.TrimSpace(h.Subject) != "" ||
		strings.TrimSpace(h.XMPLocation) != "" ||
		strings.TrimSpace(h.XMPCity) != "" ||
		strings.TrimSpace(h.XMPCountry) != "" ||
		len(h.Keywords) > 0 ||
		len(h.XMPExtra) > 0 {
		return true
	}
	base := strings.TrimSuffix(filepath.Base(h.Filename), filepath.Ext(h.Filename))
	base = strings.TrimSpace(base)
	if base == "" {
		return false
	}
	// Skip camera serial names like IMG_1234 / DSC00001 / PXL_20240101.
	if genericFilename.MatchString(base) {
		return false
	}
	return true
}

var genericFilename = regexp.MustCompile(`(?i)^(IMG|DSC|DSCN|PXL|MVIMG|VID|Screenshot|Photo)[-_ ]?\d`)

func (h PlaceHints) promptBlock() string {
	var b strings.Builder
	add := func(label, v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		fmt.Fprintf(&b, "%s: %s\n", label, v)
	}
	add("Filename", filepath.Base(h.Filename))
	add("Date", h.Date)
	add("Title", h.Title)
	add("Description", h.Description)
	add("Comment", h.Comment)
	add("Subject", h.Subject)
	if len(h.Keywords) > 0 {
		add("Keywords", strings.Join(h.Keywords, ", "))
	}
	add("XMP Location", h.XMPLocation)
	add("XMP City", h.XMPCity)
	add("XMP Country", h.XMPCountry)
	if len(h.XMPExtra) > 0 {
		add("Other tags", strings.Join(h.XMPExtra, "; "))
	}
	add("Artist", h.Artist)
	add("Camera", strings.TrimSpace(h.Make+" "+h.Model))
	add("Software", h.Software)
	return b.String()
}

func collectHints(raw []byte, x *exif.Exif, filename, date string) PlaceHints {
	h := PlaceHints{Filename: filename, Date: date}
	if x != nil {
		h.Description = exifString(x, exif.ImageDescription)
		h.Comment = exifString(x, exif.UserComment)
		h.Title = decodeXP(x, exif.XPTitle)
		h.Subject = decodeXP(x, exif.XPSubject)
		h.Artist = firstNonEmpty(exifString(x, exif.Artist), decodeXP(x, exif.XPAuthor))
		h.Software = exifString(x, exif.Software)
		h.Make = exifString(x, exif.Make)
		h.Model = exifString(x, exif.Model)
		if kw := decodeXP(x, exif.XPKeywords); kw != "" {
			h.Keywords = splitTags(kw)
		}
		if c := decodeXP(x, exif.XPComment); c != "" && h.Comment == "" {
			h.Comment = c
		}
	}
	fillXMP(&h, raw)
	return h
}

func exifString(x *exif.Exif, name exif.FieldName) string {
	t, err := x.Get(name)
	if err != nil || t == nil {
		return ""
	}
	s, err := t.StringVal()
	if err != nil {
		// Some tags (UserComment) are binary; fall back to fmt.
		s = strings.TrimSpace(t.String())
	}
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "ASCII")
	s = strings.TrimSpace(strings.Trim(s, "\x00"))
	if s == "" || onlySpaces(s) {
		return ""
	}
	return s
}

func decodeXP(x *exif.Exif, name exif.FieldName) string {
	t, err := x.Get(name)
	if err != nil || t == nil || t.Count == 0 {
		return ""
	}
	// XP* tags are UTF-16LE stored as a byte/int array.
	raw := make([]byte, 0, t.Count)
	for i := 0; i < int(t.Count); i++ {
		n, err := t.Int(i)
		if err != nil {
			break
		}
		raw = append(raw, byte(n))
	}
	if len(raw) >= 2 {
		if s := utf16LEString(raw); s != "" {
			return s
		}
	}
	return exifString(x, name)
}

func utf16LEString(b []byte) string {
	if len(b) < 2 {
		return ""
	}
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		c := uint16(b[i]) | uint16(b[i+1])<<8
		if c == 0 {
			break
		}
		u = append(u, c)
	}
	return strings.TrimSpace(string(utf16.Decode(u)))
}

func onlySpaces(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
	}
	return true
}

func splitTags(s string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ';' || r == ',' || r == '/' || r == '|'
	})
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || seen[strings.ToLower(p)] {
			continue
		}
		seen[strings.ToLower(p)] = true
		out = append(out, p)
	}
	return out
}

var (
	xmpAttr = regexp.MustCompile(`(?i)(?:photoshop:|Iptc4xmpCore:|exif:|tiff:|dc:|xmp:|aux:)?(Location|City|State|Province|Country|CountryCode|LocationCreated|LocationShown|Title|Description|Subject|HierarchicalSubject)="([^"]{1,200})"`)
	xmpTag  = regexp.MustCompile(`(?i)<(?:photoshop:|Iptc4xmpCore:|dc:|exif:)?(Location|City|State|Province|Country|CountryCode|title|description|subject)[^>]*>([^<]{1,200})</`)
	rdfLi   = regexp.MustCompile(`(?i)<rdf:li[^>]*>([^<]{1,120})</rdf:li>`)
)

func fillXMP(h *PlaceHints, raw []byte) {
	// Limit scan window — XMP packet is near the start for JPEGs.
	window := raw
	if len(window) > 256<<10 {
		window = window[:256<<10]
	}
	if !bytes.Contains(window, []byte("xmp")) && !bytes.Contains(window, []byte("XMP")) {
		return
	}
	text := string(window)

	take := func(key, val string) {
		key = strings.ToLower(key)
		val = strings.TrimSpace(htmlUnescape(val))
		if val == "" {
			return
		}
		switch key {
		case "location", "locationcreated", "locationshown":
			if h.XMPLocation == "" {
				h.XMPLocation = val
			}
		case "city":
			if h.XMPCity == "" {
				h.XMPCity = val
			}
		case "country", "countrycode":
			if h.XMPCountry == "" {
				h.XMPCountry = val
			}
		case "state", "province":
			h.XMPExtra = appendUnique(h.XMPExtra, val)
		case "title":
			if h.Title == "" {
				h.Title = val
			}
		case "description":
			if h.Description == "" {
				h.Description = val
			}
		case "subject", "hierarchicalsubject":
			for _, p := range splitTags(strings.ReplaceAll(val, "|", ",")) {
				h.Keywords = appendUnique(h.Keywords, p)
			}
		}
	}

	for _, m := range xmpAttr.FindAllStringSubmatch(text, 40) {
		take(m[1], m[2])
	}
	for _, m := range xmpTag.FindAllStringSubmatch(text, 40) {
		take(m[1], m[2])
	}
	// dc:subject bag items often hold album/people tags.
	for _, m := range rdfLi.FindAllStringSubmatch(text, 30) {
		v := strings.TrimSpace(htmlUnescape(m[1]))
		if v != "" && !strings.Contains(v, "://") {
			h.Keywords = appendUnique(h.Keywords, v)
		}
	}
}

func appendUnique(list []string, v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return list
	}
	low := strings.ToLower(v)
	for _, x := range list {
		if strings.ToLower(x) == low {
			return list
		}
	}
	return append(list, v)
}

func htmlUnescape(s string) string {
	r := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&apos;", "'",
	)
	return r.Replace(s)
}
