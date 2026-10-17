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
}

type Hub struct {
	store *Store
	mu    sync.Mutex
	conns map[*websocket.Conn]struct{}
}

func NewHub(store *Store) *Hub {
	return &Hub{store: store, conns: make(map[*websocket.Conn]struct{})}
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
	if err := conn.WriteJSON(Envelope{Type: "snapshot", Data: snap}); err != nil {
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

func (h *Hub) broadcast(msg Envelope) {
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
