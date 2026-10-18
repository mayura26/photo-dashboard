package gcal

import (
	"testing"
	"time"

	"google.golang.org/api/calendar/v3"
)

func TestNormalizeEventsPersonal(t *testing.T) {
	cal := CalendarRef{ID: "cal1", Source: "personal", People: []string{"alex"}}
	items := []*calendar.Event{
		{
			Id:      "ev1",
			Summary: "Gym",
			Start:   &calendar.EventDateTime{DateTime: "2026-10-18T15:00:00-07:00"},
			End:     &calendar.EventDateTime{DateTime: "2026-10-18T16:00:00-07:00"},
			Location: "YMCA",
		},
		{Id: "cancelled", Status: "cancelled", Summary: "Nope"},
	}
	out := NormalizeEvents(items, cal)
	if len(out) != 1 {
		t.Fatalf("got %d events, want 1", len(out))
	}
	e := out[0]
	if e["id"] != "alex:ev1" {
		t.Errorf("id = %v", e["id"])
	}
	if e["title"] != "Gym" {
		t.Errorf("title = %v", e["title"])
	}
	if e["source"] != "personal" {
		t.Errorf("source = %v", e["source"])
	}
	people, _ := e["people"].([]string)
	if len(people) != 1 || people[0] != "alex" {
		t.Errorf("people = %v", e["people"])
	}
	if e["loc"] != "YMCA" {
		t.Errorf("loc = %v", e["loc"])
	}
	if e["allDay"] != nil {
		t.Errorf("unexpected allDay")
	}
	start, _ := e["start"].(string)
	if _, err := time.Parse(time.RFC3339Nano, start); err != nil {
		t.Errorf("start parse: %v", err)
	}
}

func TestNormalizeEventsAllDayFamily(t *testing.T) {
	cal := CalendarRef{ID: "fam", Source: "family", People: []string{"alex", "jordan"}}
	items := []*calendar.Event{
		{
			Id:      "day1",
			Summary: "Holiday",
			Start:   &calendar.EventDateTime{Date: "2026-12-25"},
			End:     &calendar.EventDateTime{Date: "2026-12-26"},
		},
	}
	out := NormalizeEvents(items, cal)
	if len(out) != 1 {
		t.Fatalf("got %d events, want 1", len(out))
	}
	e := out[0]
	if e["id"] != "family:day1" {
		t.Errorf("id = %v", e["id"])
	}
	if e["allDay"] != true {
		t.Errorf("allDay = %v", e["allDay"])
	}
	if e["source"] != "family" {
		t.Errorf("source = %v", e["source"])
	}
	people, _ := e["people"].([]string)
	if len(people) != 2 {
		t.Errorf("people = %v", e["people"])
	}
}

func TestTruncateNotes(t *testing.T) {
	cal := CalendarRef{ID: "c", Source: "personal", People: []string{"jordan"}}
	long := make([]rune, maxNotesRunes+50)
	for i := range long {
		long[i] = 'a'
	}
	items := []*calendar.Event{{
		Id:          "n1",
		Summary:     "Note",
		Description: string(long),
		Start:       &calendar.EventDateTime{DateTime: "2026-10-18T10:00:00Z"},
		End:         &calendar.EventDateTime{DateTime: "2026-10-18T11:00:00Z"},
	}}
	out := NormalizeEvents(items, cal)
	notes, _ := out[0]["notes"].(string)
	if len([]rune(notes)) != maxNotesRunes+1 { // truncated + ellipsis
		t.Errorf("notes rune count = %d", len([]rune(notes)))
	}
}
