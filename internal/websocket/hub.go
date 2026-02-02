package websocket

import (
	"log/slog"
	"sync"
)

type Hub struct {
	clients         map[*Client]bool
	Broadcast       chan []byte
	register        chan *Client
	unregister      chan *Client
	mu              sync.RWMutex
	decisionHandler DecisionHandler
}

func NewHub() *Hub {
	return &Hub{
		Broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) SetDecisionHandler(handler DecisionHandler) {
	h.decisionHandler = handler
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			slog.Info("Client registered", "totalClients", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				slog.Info("Client unregistered", "totalClients", len(h.clients))
			}
			h.mu.Unlock()

		case message := <-h.Broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}
