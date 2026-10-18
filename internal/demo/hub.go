package demo

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Envelope struct {
	Type    string         `json:"type"`
	Name    string         `json:"name,omitempty"`
	Payload map[string]any `json:"payload,omitempty"`
	Data    any            `json:"data,omitempty"`
	Version string         `json:"version,omitempty"`
}

// PmdActions handles Plan My Day complete/schedule actions (network I/O outside Apply).
type PmdActions interface {
	Handle(name string, payload map[string]any) error
}

type Hub struct {
	store   *Store
	version string
	pmd     PmdActions
	mu      sync.Mutex
	conns   map[*websocket.Conn]struct{}
}

func NewHub(store *Store, version string) *Hub {
	if version == "" {
		version = "dev"
	}
	return &Hub{store: store, version: version, conns: make(map[*websocket.Conn]struct{})}
}

// SetPmdActions wires Plan My Day mutation handling.
func (h *Hub) SetPmdActions(a PmdActions) {
	h.pmd = a
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("ws upgrade:", err)
		return
	}
	h.mu.Lock()
	h.conns[conn] = struct{}{}
	h.mu.Unlock()

	snap := h.store.Snapshot()
	if err := conn.WriteJSON(h.envelope(Envelope{Type: "snapshot", Data: snap})); err != nil {
		log.Println("ws snapshot:", err)
		h.drop(conn)
		return
	}

	defer h.drop(conn)
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg Envelope
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		if msg.Type != "action" || msg.Name == "" {
			continue
		}
		if msg.Payload == nil {
			msg.Payload = map[string]any{}
		}
		if msg.Name == "completePmdTask" || msg.Name == "schedulePmdTask" {
			if h.pmd == nil {
				_ = conn.WriteJSON(h.envelope(Envelope{
					Type:    "actionError",
					Name:    msg.Name,
					Payload: map[string]any{"error": "Plan My Day is not configured"},
				}))
				continue
			}
			if err := h.pmd.Handle(msg.Name, msg.Payload); err != nil {
				log.Printf("pmday action %s: %v", msg.Name, err)
				_ = conn.WriteJSON(h.envelope(Envelope{
					Type:    "actionError",
					Name:    msg.Name,
					Payload: map[string]any{"error": err.Error()},
				}))
				continue
			}
			h.BroadcastSnapshot()
			continue
		}
		snap, eventName, err := h.store.Apply(msg.Name, msg.Payload)
		if err != nil {
			continue
		}
		h.broadcast(Envelope{Type: "snapshot", Data: snap})
		if eventName != "" {
			h.broadcast(Envelope{Type: "event", Name: eventName})
		}
	}
}

func (h *Hub) drop(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.conns, conn)
	h.mu.Unlock()
	_ = conn.Close()
}

func (h *Hub) envelope(msg Envelope) Envelope {
	msg.Version = h.version
	return msg
}

func (h *Hub) broadcast(msg Envelope) {
	msg = h.envelope(msg)
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.conns {
		if err := c.WriteJSON(msg); err != nil {
			_ = c.Close()
			delete(h.conns, c)
		}
	}
}

// BroadcastSnapshot pushes the current store snapshot to all connected clients.
func (h *Hub) BroadcastSnapshot() {
	h.broadcast(Envelope{Type: "snapshot", Data: h.store.Snapshot()})
}
