package gcal

import (
	"context"
	"log"
	"time"
)

// Updater applies calendar events into the dashboard store.
type Updater interface {
	SetEvents(events []map[string]any)
}

// Notifier pushes an updated snapshot to connected clients.
type Notifier interface {
	BroadcastSnapshot()
}

// Poller periodically fetches Google Calendar events and updates the store.
type Poller struct {
	Client   *Client
	Cals     []CalendarRef
	Interval time.Duration
	Past     time.Duration
	Future   time.Duration
	Store    Updater
	Hub      Notifier
}

// Start runs an immediate fetch, then polls on Interval until the process exits.
func (p *Poller) Start() {
	if p.Interval <= 0 {
		p.Interval = 15 * time.Minute
	}
	if p.Past <= 0 {
		p.Past = 7 * 24 * time.Hour
	}
	if p.Future <= 0 {
		p.Future = 60 * 24 * time.Hour
	}
	go p.loop()
}

func (p *Poller) loop() {
	p.tick()
	t := time.NewTicker(p.Interval)
	defer t.Stop()
	for range t.C {
		p.tick()
	}
}

func (p *Poller) tick() {
	if p.Client == nil || p.Store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	now := time.Now()
	events, err := p.Client.Fetch(ctx, p.Cals, now.Add(-p.Past), now.Add(p.Future))
	if err != nil {
		log.Printf("gcal: fetch failed: %v", err)
		return
	}
	p.Store.SetEvents(events)
	if p.Hub != nil {
		p.Hub.BroadcastSnapshot()
	}
	log.Printf("gcal: updated %d events from %d calendar(s)", len(events), len(p.Cals))
}
