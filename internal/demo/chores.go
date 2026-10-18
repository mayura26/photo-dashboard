package demo

import (
	"encoding/json"
	"strings"
	"time"
)

// nowFunc is the clock used for chore day logic (overridable in tests).
var nowFunc = time.Now

func todayKey() string {
	return nowFunc().In(time.Local).Format("2006-01-02")
}

func choreDueDay(c map[string]any, fallback string) string {
	day, _ := c["day"].(string)
	day = strings.TrimSpace(day)
	if day == "" {
		return fallback
	}
	if _, err := time.Parse("2006-01-02", day); err != nil {
		return fallback
	}
	return day
}

func isRecurring(c map[string]any) bool {
	v, ok := c["recurring"].(bool)
	return ok && v
}

func choreDoneOn(c map[string]any) string {
	s, _ := c["doneOn"].(string)
	return strings.TrimSpace(s)
}

// choreAppliesOn reports whether c counts toward person who on date day.
func choreAppliesOn(c map[string]any, who, day string) bool {
	w, _ := c["who"].(string)
	if w != who {
		return false
	}
	due := choreDueDay(c, day)
	if due > day {
		return false
	}
	done, _ := c["done"].(bool)
	if !isRecurring(c) && done {
		on := choreDoneOn(c)
		if on != "" && on < day {
			return false
		}
	}
	return true
}

// ensureChoreDayLocked rolls recurring chores past their done day and refreshes week.
// Returns true if mutable state changed (needs persist).
func (s *Store) ensureChoreDayLocked() bool {
	today := todayKey()
	changed := false
	if s.choreDay != today {
		if s.rollRecurringLocked(today) {
			changed = true
		}
		s.choreDay = today
	}
	s.recomputeWeekLocked(today)
	return changed
}

func (s *Store) rollRecurringLocked(today string) bool {
	changed := false
	for _, c := range s.snap.Chores {
		if !isRecurring(c) {
			continue
		}
		done, _ := c["done"].(bool)
		if !done {
			continue
		}
		on := choreDoneOn(c)
		if on == "" || on >= today {
			continue
		}
		c["done"] = false
		delete(c, "doneBy")
		delete(c, "doneOn")
		changed = true
	}
	return changed
}

func (s *Store) refreshHitsLocked(who, day string) {
	if who == "" || who == "any" || day == "" {
		return
	}
	if s.weekHits == nil {
		s.weekHits = map[string]map[string]bool{}
	}
	if allChoresDoneFor(s.snap.Chores, who, day) {
		if s.weekHits[who] == nil {
			s.weekHits[who] = map[string]bool{}
		}
		s.weekHits[who][day] = true
	} else if byDay := s.weekHits[who]; byDay != nil {
		delete(byDay, day)
	}
}

func allChoresDoneFor(chores []map[string]any, who, day string) bool {
	n := 0
	for _, c := range chores {
		if !choreAppliesOn(c, who, day) {
			continue
		}
		n++
		done, _ := c["done"].(bool)
		if !done || choreDoneOn(c) != day {
			return false
		}
	}
	return n > 0
}

func (s *Store) recomputeWeekLocked(today string) {
	ids := s.personIDsLocked()
	week := make(map[string][]int, len(ids))
	monday := mondayOf(today)
	for _, id := range ids {
		arr := make([]int, 7)
		for i := 0; i < 7; i++ {
			d := monday.AddDate(0, 0, i).Format("2006-01-02")
			if s.weekHits[id][d] {
				arr[i] = 1
			}
		}
		week[id] = arr
	}
	raw, _ := json.Marshal(week)
	s.snap.Week = raw
}

func mondayOf(day string) time.Time {
	t, err := time.ParseInLocation("2006-01-02", day, time.Local)
	if err != nil {
		t = nowFunc().In(time.Local)
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
	}
	// Go: Sunday=0 … Saturday=6. Shift so Monday=0.
	wd := int(t.Weekday())
	offset := (wd + 6) % 7
	return t.AddDate(0, 0, -offset)
}

func (s *Store) personIDsLocked() []string {
	var people []struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(s.snap.People, &people)
	ids := make([]string, 0, len(people))
	for _, p := range people {
		if p.ID != "" {
			ids = append(ids, p.ID)
		}
	}
	return ids
}

func (s *Store) delayChore(p map[string]any) {
	id, _ := p["id"].(string)
	days := intNum(p["days"])
	if id == "" || days < 1 || days > 3 {
		return
	}
	today := todayKey()
	target := nowFunc().In(time.Local).AddDate(0, 0, days).Format("2006-01-02")
	for _, c := range s.snap.Chores {
		if c["id"] != id {
			continue
		}
		c["day"] = target
		who, _ := c["who"].(string)
		s.refreshHitsLocked(who, today)
		s.recomputeWeekLocked(today)
		return
	}
}
