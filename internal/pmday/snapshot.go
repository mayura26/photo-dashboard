package pmday

import (
	"strings"
	"time"
)

// NormalizeItem maps a Plan My Day agenda item into the dashboard event shape.
func NormalizeItem(item agendaItem, person string) (map[string]any, bool) {
	id := strings.TrimSpace(item.ID)
	if id == "" {
		return nil, false
	}
	start, err1 := time.Parse(time.RFC3339, item.ScheduledStart)
	if err1 != nil {
		start, err1 = time.Parse(time.RFC3339Nano, item.ScheduledStart)
	}
	end, err2 := time.Parse(time.RFC3339, item.ScheduledEnd)
	if err2 != nil {
		end, err2 = time.Parse(time.RFC3339Nano, item.ScheduledEnd)
	}
	if err1 != nil || err2 != nil {
		return nil, false
	}
	if !end.After(start) {
		end = start.Add(30 * time.Minute)
	}
	title := strings.TrimSpace(item.Title)
	if title == "" {
		title = "(No title)"
	}
	if person == "" {
		person = "mayura"
	}
	status := strings.ToLower(strings.TrimSpace(item.Status))
	if status == "" {
		status = "pending"
	}
	e := map[string]any{
		"id":     "pmday:" + id,
		"title":  title,
		"start":  start.UTC().Format(time.RFC3339Nano),
		"end":    end.UTC().Format(time.RFC3339Nano),
		"people": []string{person},
		"source": "pmday",
		"status": status,
	}
	if tt := strings.ToLower(strings.TrimSpace(item.TaskType)); tt != "" {
		e["taskType"] = tt
	}
	if item.Priority > 0 {
		e["priority"] = item.Priority
	}
	if item.Locked {
		e["locked"] = true
	}
	if item.DueDate != nil && strings.TrimSpace(*item.DueDate) != "" {
		e["dueDate"] = strings.TrimSpace(*item.DueDate)
	}
	if item.Duration != nil && *item.Duration > 0 {
		e["duration"] = *item.Duration
	}
	return e, true
}
