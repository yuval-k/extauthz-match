package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	applog "github.com/yuval/extauth-match/internal/log"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for demo
	},
}

type Tenant struct {
	tenantID string
	server   *websocket.Conn
	client   *websocket.Conn
	mu       sync.RWMutex
}

type Relay struct {
	tenants map[string]*Tenant
	mu      sync.RWMutex
}

func NewRelay() *Relay {
	return &Relay{
		tenants: make(map[string]*Tenant),
	}
}

func (r *Relay) getTenant(tenantID string) *Tenant {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tenant, exists := r.tenants[tenantID]; exists {
		return tenant
	}

	tenant := &Tenant{tenantID: tenantID}
	r.tenants[tenantID] = tenant
	return tenant
}

func (r *Relay) handleServerConnect(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	tenantID := vars["tenantID"]

	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		slog.Error("Server upgrade failed", "tenantID", tenantID, "error", err)
		return
	}

	tenant := r.getTenant(tenantID)
	tenant.mu.Lock()
	tenant.server = conn
	tenant.mu.Unlock()

	slog.Info("Authz server connected", "tenantID", tenantID)

	// Read from server and forward to client
	go r.forwardServerToClient(tenant)
}

func (r *Relay) handleClientConnect(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	tenantID := vars["tenantID"]

	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		slog.Error("Client upgrade failed", "tenantID", tenantID, "error", err)
		return
	}

	tenant := r.getTenant(tenantID)
	tenant.mu.Lock()

	// Disconnect existing client if any
	if tenant.client != nil {
		tenant.client.Close()
	}
	tenant.client = conn
	tenant.mu.Unlock()

	slog.Info("Browser client connected", "tenantID", tenantID)

	// Read from client and forward to server
	go r.forwardClientToServer(tenant)
}

func (r *Relay) forwardServerToClient(tenant *Tenant) {
	defer func() {
		tenant.mu.Lock()
		if tenant.server != nil {
			tenant.server.Close()
			tenant.server = nil
		}
		tenant.mu.Unlock()
		slog.Info("Authz server disconnected", "tenantID", tenant.tenantID)
	}()

	for {
		tenant.mu.RLock()
		server := tenant.server
		tenant.mu.RUnlock()

		if server == nil {
			return
		}

		messageType, message, err := server.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("Server read error", "tenantID", tenant.tenantID, "error", err)
			}
			return
		}

		// Forward to client
		tenant.mu.RLock()
		client := tenant.client
		tenant.mu.RUnlock()

		if client != nil {
			if err := client.WriteMessage(messageType, message); err != nil {
				slog.Error("Failed to forward to client", "tenantID", tenant.tenantID, "error", err)
			} else {
				slog.Info("Forwarded bytes from server to client", "bytes", len(message), "tenantID", tenant.tenantID)
			}
		}
	}
}

func (r *Relay) forwardClientToServer(tenant *Tenant) {
	defer func() {
		tenant.mu.Lock()
		if tenant.client != nil {
			tenant.client.Close()
			tenant.client = nil
		}
		tenant.mu.Unlock()
		slog.Info("Browser client disconnected", "tenantID", tenant.tenantID)
	}()

	for {
		tenant.mu.RLock()
		client := tenant.client
		tenant.mu.RUnlock()

		if client == nil {
			return
		}

		messageType, message, err := client.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("Client read error", "tenantID", tenant.tenantID, "error", err)
			}
			return
		}

		// Forward to server
		tenant.mu.RLock()
		server := tenant.server
		tenant.mu.RUnlock()

		if server != nil {
			if err := server.WriteMessage(messageType, message); err != nil {
				slog.Error("Failed to forward to server", "tenantID", tenant.tenantID, "error", err)
			} else {
				slog.Info("Forwarded bytes from client to server", "bytes", len(message), "tenantID", tenant.tenantID)
			}
		}
	}
}

func main() {
	applog.SetupLogging()

	relay := NewRelay()

	router := mux.NewRouter()
	router.HandleFunc("/ws/server/{tenantID}", relay.handleServerConnect)
	router.HandleFunc("/ws/client/{tenantID}", relay.handleClientConnect)

	// Serve static HTML for client
	router.HandleFunc("/s/{tenantID}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/static/index.html")
	})

	bindAddr := ":9090"
	if envPort := os.Getenv("PORT"); envPort != "" {
		bindAddr = ":" + envPort
	}

	server := &http.Server{
		Addr:    bindAddr,
		Handler: router,
	}

	go func() {
		slog.Info("Relay server listening", "address", ":9090")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Failed to start relay server", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	slog.Info("Shutting down relay server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server.Shutdown(ctx)
	slog.Info("Relay server shutdown complete")
}
