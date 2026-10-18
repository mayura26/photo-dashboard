package pmday

import (
	"context"
	"log"
	"time"
)

// Updater applies Plan My Day events into the dashboard store.
type Updater interface {
	SetSourceEvents(source string, events []map[string]any)
}

// Notifier pushes an updated snapshot to connected clients.
type Notifier interface {
	BroadcastSnapshot()
}

// Poller periodically fetches the Plan My Day agenda and updates the store.
type Poller struct {
	Client   *Client
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
	p.Tick()
	t := time.NewTicker(p.Interval)
	defer t.Stop()
	for range t.C {
		p.Tick()
	}
}

// Tick fetches the agenda once and updates the store.
func (p *Poller) Tick() {
	if p.Client == nil || p.Store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	now := time.Now()
	events, err := p.Client.FetchAgenda(ctx, now.Add(-p.Past), now.Add(p.Future))
	if err != nil {
		log.Printf("pmday: fetch failed: %v", err)
		return
	}
	p.Store.SetSourceEvents("pmday", events)
	if p.Hub != nil {
		p.Hub.BroadcastSnapshot()
	}
	log.Printf("pmday: updated %d events", len(events))
}
