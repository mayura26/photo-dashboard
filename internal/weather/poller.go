package weather

import (
	"encoding/json"
	"log"
	"time"
)

// Updater applies weather JSON into the dashboard store.
type Updater interface {
	SetWeather(raw json.RawMessage)
}

// Notifier pushes an updated snapshot to connected clients.
type Notifier interface {
	BroadcastSnapshot()
}

// Poller periodically fetches weather and updates the store.
type Poller struct {
	Client   *Client
	Lat, Lon float64
	Interval time.Duration
	Store    Updater
	Hub      Notifier
}

// Start runs an immediate fetch, then polls on Interval until the process exits.
func (p *Poller) Start() {
	if p.Client == nil {
		p.Client = NewClient()
	}
	if p.Interval <= 0 {
		p.Interval = 15 * time.Minute
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
	raw, err := p.Client.Fetch(p.Lat, p.Lon)
	if err != nil {
		log.Printf("weather: fetch failed: %v", err)
		return
	}
	p.Store.SetWeather(raw)
	if p.Hub != nil {
		p.Hub.BroadcastSnapshot()
	}
	log.Printf("weather: updated from Open-Meteo (%.4f, %.4f)", p.Lat, p.Lon)
}
