package pmday

import (
	"testing"
	"time"
)

func TestNormalizeItem(t *testing.T) {
	item := agendaItem{
		ID:             "task_abc",
		Title:          "Standup",
		TaskType:       "event",
		Status:         "pending",
		Priority:       3,
		ScheduledStart: "2026-04-30T23:00:00.000Z",
		ScheduledEnd:   "2026-04-30T23:30:00.000Z",
	}
	due := "2026-05-01T17:00:00.000Z"
	item.DueDate = &due

	ev, ok := NormalizeItem(item, "mayura")
	if !ok {
		t.Fatal("expected ok")
	}
	if ev["id"] != "pmday:task_abc" {
		t.Fatalf("id: %v", ev["id"])
	}
	if ev["source"] != "pmday" {
		t.Fatalf("source: %v", ev["source"])
	}
	if ev["status"] != "pending" {
		t.Fatalf("status: %v", ev["status"])
	}
	people, _ := ev["people"].([]string)
	if len(people) != 1 || people[0] != "mayura" {
		t.Fatalf("people: %#v", ev["people"])
	}
	if ev["dueDate"] != due {
		t.Fatalf("dueDate: %v", ev["dueDate"])
	}
	start, _ := ev["start"].(string)
	if _, err := time.Parse(time.RFC3339Nano, start); err != nil {
		t.Fatalf("start parse: %v", err)
	}
}

func TestNormalizeItemRejectsBadTimes(t *testing.T) {
	_, ok := NormalizeItem(agendaItem{ID: "x", Title: "t", ScheduledStart: "nope", ScheduledEnd: "nope"}, "mayura")
	if ok {
		t.Fatal("expected reject")
	}
}

func TestDateChunksRespectsMaxDays(t *testing.T) {
	loc := time.UTC
	start := time.Date(2026, 5, 1, 12, 0, 0, 0, loc)
	end := time.Date(2026, 7, 10, 12, 0, 0, 0, loc) // ~70 days
	chunks := DateChunks(start, end, 31)
	if len(chunks) < 3 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		days := int(c.End.Sub(c.Start).Hours()/24) + 1
		if days > 31 {
			t.Fatalf("chunk %d spans %d days: %v–%v", i, days, c.Start, c.End)
		}
		if i > 0 && !c.Start.Equal(chunks[i-1].End.AddDate(0, 0, 1)) {
			t.Fatalf("gap between chunks %d and %d", i-1, i)
		}
	}
	if !chunks[0].Start.Equal(time.Date(2026, 5, 1, 0, 0, 0, 0, loc)) {
		t.Fatalf("first start %v", chunks[0].Start)
	}
	if !chunks[len(chunks)-1].End.Equal(time.Date(2026, 7, 10, 0, 0, 0, 0, loc)) {
		t.Fatalf("last end %v", chunks[len(chunks)-1].End)
	}
}

func TestStripPrefix(t *testing.T) {
	if StripPrefix("pmday:task_1") != "task_1" {
		t.Fatal(StripPrefix("pmday:task_1"))
	}
	if StripPrefix("task_1") != "task_1" {
		t.Fatal(StripPrefix("task_1"))
	}
}
