package pmday

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxAgendaDays = 31

// Client talks to the Plan My Day external API.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	Person     string // dashboard person id stamped onto events
}

// NewClient builds a Plan My Day API client. baseURL should be the app origin (no trailing slash).
func NewClient(baseURL, apiKey string) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	apiKey = strings.TrimSpace(apiKey)
	if baseURL == "" || apiKey == "" {
		return nil, fmt.Errorf("pmday: missing PLAN_MY_DAY_URL or PLAN_MY_DAY_API_KEY")
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("pmday: invalid PLAN_MY_DAY_URL: %w", err)
	}
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		Person: "mayura",
	}, nil
}

type agendaResponse struct {
	Timezone string      `json:"timezone"`
	Start    string      `json:"start"`
	End      string      `json:"end"`
	Days     []agendaDay `json:"days"`
}

type agendaDay struct {
	Date  string       `json:"date"`
	Items []agendaItem `json:"items"`
}

type agendaItem struct {
	ID                   string  `json:"id"`
	Title                string  `json:"title"`
	TaskType             string  `json:"task_type"`
	Status               string  `json:"status"`
	Priority             int     `json:"priority"`
	Duration             *int    `json:"duration"`
	ScheduledStart       string  `json:"scheduled_start"`
	ScheduledEnd         string  `json:"scheduled_end"`
	DueDate              *string `json:"due_date"`
	GroupID              *string `json:"group_id"`
	ParentTaskID         *string `json:"parent_task_id"`
	EnergyLevelRequired  int     `json:"energy_level_required"`
	Locked               bool    `json:"locked"`
}

// FetchAgenda returns normalized dashboard events for [timeMin, timeMax], chunked to ≤31 days.
func (c *Client) FetchAgenda(ctx context.Context, timeMin, timeMax time.Time) ([]map[string]any, error) {
	if c == nil {
		return nil, fmt.Errorf("pmday: client not initialized")
	}
	ranges := DateChunks(timeMin, timeMax, maxAgendaDays)
	var out []map[string]any
	seen := map[string]bool{}
	for _, r := range ranges {
		resp, err := c.fetchAgendaRange(ctx, r.Start, r.End)
		if err != nil {
			return nil, err
		}
		for _, day := range resp.Days {
			for _, item := range day.Items {
				ev, ok := NormalizeItem(item, c.Person)
				if !ok {
					continue
				}
				id, _ := ev["id"].(string)
				if id == "" || seen[id] {
					continue
				}
				seen[id] = true
				out = append(out, ev)
			}
		}
	}
	return out, nil
}

func (c *Client) fetchAgendaRange(ctx context.Context, start, end time.Time) (*agendaResponse, error) {
	q := url.Values{}
	q.Set("start", start.Format("2006-01-02"))
	q.Set("end", end.Format("2006-01-02"))
	u := c.BaseURL + "/api/tasks/agenda?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pmday: agenda: %w", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pmday: agenda %s: %s", res.Status, truncateErr(body))
	}
	var out agendaResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("pmday: agenda decode: %w", err)
	}
	return &out, nil
}

// Complete marks a Plan My Day task completed. taskID is the raw API id (no pmday: prefix).
func (c *Client) Complete(ctx context.Context, taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return fmt.Errorf("pmday: empty task id")
	}
	u := c.BaseURL + "/api/tasks/" + url.PathEscape(taskID) + "/complete"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return err
	}
	c.setAuth(req)
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("pmday: complete: %w", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("pmday: complete %s: %s", res.Status, truncateErr(body))
	}
	return nil
}

type scheduleRequest struct {
	Mode string `json:"mode"`
}

type scheduleResponse struct {
	Success  bool           `json:"success"`
	Mode     string         `json:"mode"`
	Task     map[string]any `json:"task"`
	Feedback []any          `json:"feedback"`
}

// Schedule reschedules a task with the given mode. Returns the API task object when present.
func (c *Client) Schedule(ctx context.Context, taskID, mode string) (map[string]any, error) {
	taskID = strings.TrimSpace(taskID)
	mode = strings.TrimSpace(mode)
	if taskID == "" || mode == "" {
		return nil, fmt.Errorf("pmday: task id and mode required")
	}
	payload, err := json.Marshal(scheduleRequest{Mode: mode})
	if err != nil {
		return nil, err
	}
	u := c.BaseURL + "/api/tasks/" + url.PathEscape(taskID) + "/schedule"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pmday: schedule: %w", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pmday: schedule %s: %s", res.Status, truncateErr(body))
	}
	var out scheduleResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("pmday: schedule decode: %w", err)
	}
	return out.Task, nil
}

func (c *Client) setAuth(req *http.Request) {
	key := c.APIKey
	if !strings.HasPrefix(key, "Bearer ") {
		key = "Bearer " + key
	}
	req.Header.Set("Authorization", key)
}

func truncateErr(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200] + "…"
	}
	if s == "" {
		return "(empty body)"
	}
	return s
}

// StripPrefix removes the dashboard "pmday:" id prefix.
func StripPrefix(id string) string {
	return strings.TrimPrefix(id, "pmday:")
}
