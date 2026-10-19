package payouts

import "strings"

// Firm is a prop firm the model and prefix matcher can choose.
type Firm struct {
	ID       string
	Name     string
	Prefixes []string
}

// MatchFirm returns the firm whose prefix is the longest case-insensitive
// prefix of account. Empty when nothing matches. Empty prefixes never match.
func MatchFirm(account string, firms []Firm) string {
	account = strings.ToUpper(strings.TrimSpace(account))
	if account == "" {
		return ""
	}
	bestID := ""
	bestLen := 0
	for _, f := range firms {
		for _, p := range f.Prefixes {
			p = strings.ToUpper(strings.TrimSpace(p))
			if p == "" || len(p) <= bestLen {
				continue
			}
			if strings.HasPrefix(account, p) {
				bestID = f.ID
				bestLen = len(p)
			}
		}
	}
	return bestID
}

// AssignFirm prefers a prefix hit. Otherwise it keeps modelFirm when that
// value is a known firm id or firm name. Otherwise the payout stays unknown.
func AssignFirm(account, modelFirm string, firms []Firm) string {
	if id := MatchFirm(account, firms); id != "" {
		return id
	}
	return knownFirm(modelFirm, firms)
}

func knownFirm(idOrName string, firms []Firm) string {
	idOrName = strings.TrimSpace(idOrName)
	if idOrName == "" {
		return ""
	}
	for _, f := range firms {
		if f.ID == idOrName {
			return f.ID
		}
	}
	for _, f := range firms {
		if strings.EqualFold(f.Name, idOrName) {
			return f.ID
		}
	}
	return ""
}

// FirmsFromMaps reads id, name, and prefixes off snapshot records.
func FirmsFromMaps(list []map[string]any) []Firm {
	out := make([]Firm, 0, len(list))
	for _, m := range list {
		id, _ := m["id"].(string)
		name, _ := m["name"].(string)
		f := Firm{ID: id, Name: name, Prefixes: prefixList(m["prefixes"])}
		if f.ID == "" {
			continue
		}
		out = append(out, f)
	}
	return out
}

func prefixList(v any) []string {
	switch t := v.(type) {
	case []string:
		return append([]string(nil), t...)
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
