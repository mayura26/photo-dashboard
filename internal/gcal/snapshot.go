package gcal

import (
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/api/calendar/v3"
)

const maxNotesRunes = 500

// NormalizeEvents maps Google Calendar events into the dashboard event shape.
func NormalizeEvents(items []*calendar.Event, cal CalendarRef) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, ev := range items {
		if ev == nil || ev.Status == "cancelled" {
			continue
		}
		mapped, ok := normalizeOne(ev, cal)
		if !ok {
			continue
		}
		out = append(out, mapped)
	}
	return out
}

func normalizeOne(ev *calendar.Event, cal CalendarRef) (map[string]any, bool) {
	start, end, allDay, ok := parseBounds(ev)
	if !ok {
		return nil, false
	}
	title := strings.TrimSpace(ev.Summary)
	if title == "" {
		title = "(No title)"
	}
	id := ev.Id
	if id == "" {
		return nil, false
	}
	// Prefix with calendar key so personal + family never collide on shared ids.
	prefix := cal.Source
	if len(cal.People) == 1 {
		prefix = cal.People[0]
	} else if cal.Source == "family" {
		prefix = "family"
	}
	e := map[string]any{
		"id":     prefix + ":" + id,
		"title":  title,
		"start":  start.UTC().Format(time.RFC3339Nano),
		"end":    end.UTC().Format(time.RFC3339Nano),
		"people": append([]string(nil), cal.People...),
		"source": cal.Source,
	}
	if allDay {
		e["allDay"] = true
	}
	if loc := strings.TrimSpace(ev.Location); loc != "" {
		e["loc"] = loc
	}
	if notes := truncateRunes(strings.TrimSpace(ev.Description), maxNotesRunes); notes != "" {
		e["notes"] = notes
	}
	return e, true
}

func parseBounds(ev *calendar.Event) (start, end time.Time, allDay bool, ok bool) {
	if ev.Start == nil {
		return time.Time{}, time.Time{}, false, false
	}
	if ev.Start.Date != "" {
		// All-day: Google uses exclusive end date (YYYY-MM-DD).
		s, err1 := time.ParseInLocation("2006-01-02", ev.Start.Date, time.Local)
		if err1 != nil {
			return time.Time{}, time.Time{}, false, false
		}
		endDate := ev.Start.Date
		if ev.End != nil && ev.End.Date != "" {
			endDate = ev.End.Date
		}
		e, err2 := time.ParseInLocation("2006-01-02", endDate, time.Local)
		if err2 != nil {
			return time.Time{}, time.Time{}, false, false
		}
		if !e.After(s) {
			e = s.Add(24 * time.Hour)
		}
		return s, e, true, true
	}
	if ev.Start.DateTime == "" {
		return time.Time{}, time.Time{}, false, false
	}
	s, err := time.Parse(time.RFC3339, ev.Start.DateTime)
	if err != nil {
		return time.Time{}, time.Time{}, false, false
	}
	e := s.Add(time.Hour)
	if ev.End != nil && ev.End.DateTime != "" {
		if t, err := time.Parse(time.RFC3339, ev.End.DateTime); err == nil {
			e = t
		}
	}
	return s, e, false, true
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return s
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "…"
}
