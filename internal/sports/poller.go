package sports

import (
	"encoding/json"
	"log"
	"time"
)

// Updater applies sports JSON into the dashboard store.
type Updater interface {
	SetSports(raw json.RawMessage)
}

// Notifier pushes an updated snapshot to connected clients.
type Notifier interface {
	BroadcastSnapshot()
}

// Poller periodically fetches sports fixtures and updates the store.
type Poller struct {
	Client   *Client
	Watches  []Resolved
	Interval time.Duration
	Store    Updater
	Hub      Notifier
	Now      func() time.Time // optional; defaults to time.Now
}

// Start runs an immediate fetch, then polls on Interval until the process exits.
func (p *Poller) Start() {
	if p.Client == nil {
		p.Client = NewClient("")
	}
	if p.Interval <= 0 {
		p.Interval = 30 * time.Minute
	}
	if p.Now == nil {
		p.Now = time.Now
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
	raw, err := p.fetch()
	if err != nil {
		log.Printf("sports: fetch failed: %v", err)
		return
	}
	p.Store.SetSports(raw)
	if p.Hub != nil {
		p.Hub.BroadcastSnapshot()
	}
	log.Printf("sports: updated %d watch item(s)", len(p.Watches))
}

func (p *Poller) fetch() (json.RawMessage, error) {
	now := p.Now()
	eventsByLeague := map[string][]Event{}
	for _, w := range p.Watches {
		if w.LeagueID == "" || w.Season == "" {
			continue
		}
		if _, ok := eventsByLeague[w.LeagueID]; ok {
			continue
		}
		evs, err := p.Client.EventsSeason(w.LeagueID, w.Season)
		if err != nil {
			return nil, err
		}
		eventsByLeague[w.LeagueID] = evs
	}
	fixtures := BuildFixtures(p.Watches, eventsByLeague, now)
	return BuildSnapshotJSON(fixtures)
}
