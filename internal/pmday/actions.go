package pmday

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// ActionStore is the subset of hq.Store used after complete/schedule.
type ActionStore interface {
	SetSourceEvents(source string, events []map[string]any)
	PatchSourceEvent(source, id string, patch map[string]any) bool
}

// Actions handles complete/schedule WebSocket actions against Plan My Day.
type Actions struct {
	Client *Client
	Store  ActionStore
	Past   time.Duration
	Future time.Duration
}

// Handle runs completePmdTask or schedulePmdTask. Returns an error message for the client.
func (a *Actions) Handle(name string, payload map[string]any) error {
	if a == nil || a.Client == nil || a.Store == nil {
		return fmt.Errorf("Plan My Day is not configured")
	}
	id, _ := payload["id"].(string)
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("missing task id")
	}
	taskID := StripPrefix(id)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	switch name {
	case "completePmdTask":
		if err := a.Client.Complete(ctx, taskID); err != nil {
			return err
		}
		a.Store.PatchSourceEvent("pmday", id, map[string]any{"status": "completed"})
		a.refresh(ctx)
		return nil
	case "schedulePmdTask":
		mode, _ := payload["mode"].(string)
		mode = strings.TrimSpace(mode)
		if mode == "" {
			return fmt.Errorf("missing schedule mode")
		}
		task, err := a.Client.Schedule(ctx, taskID, mode)
		if err != nil {
			return err
		}
		if task != nil {
			patch := map[string]any{"status": "pending"}
			if v, ok := task["scheduled_start"].(string); ok && v != "" {
				if t, err := parseAPITime(v); err == nil {
					patch["start"] = t.UTC().Format(time.RFC3339Nano)
				}
			}
			if v, ok := task["scheduled_end"].(string); ok && v != "" {
				if t, err := parseAPITime(v); err == nil {
					patch["end"] = t.UTC().Format(time.RFC3339Nano)
				}
			}
			if v, ok := task["status"].(string); ok && v != "" {
				patch["status"] = v
			}
			a.Store.PatchSourceEvent("pmday", id, patch)
		}
		a.refresh(ctx)
		return nil
	default:
		return fmt.Errorf("unknown pmday action %q", name)
	}
}

func (a *Actions) refresh(ctx context.Context) {
	past := a.Past
	future := a.Future
	if past <= 0 {
		past = 7 * 24 * time.Hour
	}
	if future <= 0 {
		future = 60 * 24 * time.Hour
	}
	now := time.Now()
	events, err := a.Client.FetchAgenda(ctx, now.Add(-past), now.Add(future))
	if err != nil {
		log.Printf("pmday: refresh after action failed: %v", err)
		return
	}
	a.Store.SetSourceEvents("pmday", events)
}

func parseAPITime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}
