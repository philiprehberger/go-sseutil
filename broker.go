package sseutil

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// sseClient represents a connected SSE client.
type sseClient struct {
	id     string
	events chan []byte
}

// Broker manages Server-Sent Events client connections. It supports
// broadcasting events to all connected clients, sending events to
// specific clients, and automatic keep-alive pings.
type Broker struct {
	mu        sync.RWMutex
	clients   map[string]*sseClient
	nextID    atomic.Uint64
	keepAlive time.Duration
}

// BrokerOption configures a Broker.
type BrokerOption func(*Broker)

// WithKeepAlive sets the interval for sending keep-alive comment lines
// to connected clients. The default is 30 seconds. Set to 0 to disable.
func WithKeepAlive(d time.Duration) BrokerOption {
	return func(b *Broker) {
		b.keepAlive = d
	}
}

// NewBroker creates a new SSE broker with the given options.
func NewBroker(opts ...BrokerOption) *Broker {
	b := &Broker{
		clients:   make(map[string]*sseClient),
		keepAlive: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Handler returns an http.Handler that serves as an SSE endpoint.
//
// It sets the appropriate headers (Content-Type, Cache-Control, Connection),
// registers the client, streams events, and removes the client on disconnect.
func (b *Broker) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		id := fmt.Sprintf("client-%d", b.nextID.Add(1))
		client := &sseClient{
			id:     id,
			events: make(chan []byte, 256),
		}

		b.mu.Lock()
		b.clients[id] = client
		b.mu.Unlock()

		defer func() {
			b.mu.Lock()
			delete(b.clients, id)
			b.mu.Unlock()
		}()

		// Set up keep-alive ticker if enabled.
		var ticker *time.Ticker
		var tickCh <-chan time.Time
		if b.keepAlive > 0 {
			ticker = time.NewTicker(b.keepAlive)
			tickCh = ticker.C
			defer ticker.Stop()
		}

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case data := <-client.events:
				_, err := w.Write(data)
				if err != nil {
					return
				}
				flusher.Flush()
			case <-tickCh:
				_, err := w.Write([]byte(": keepalive\n\n"))
				if err != nil {
					return
				}
				flusher.Flush()
			}
		}
	})
}

// Broadcast sends an event to all connected clients. Events are dropped
// for clients whose send buffers are full.
func (b *Broker) Broadcast(event Event) {
	data := event.Bytes()
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, client := range b.clients {
		select {
		case client.events <- data:
		default:
			// Drop event if client buffer is full.
		}
	}
}

// Send sends an event to a specific client identified by clientID.
// It returns false if the client was not found.
func (b *Broker) Send(clientID string, event Event) bool {
	data := event.Bytes()
	b.mu.RLock()
	client, ok := b.clients[clientID]
	b.mu.RUnlock()
	if !ok {
		return false
	}
	select {
	case client.events <- data:
		return true
	default:
		return false
	}
}

// ClientCount returns the number of currently connected clients.
func (b *Broker) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}
