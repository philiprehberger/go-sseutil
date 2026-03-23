package sseutil

import (
	"encoding/json"
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
	mu           sync.RWMutex
	clients      map[string]*sseClient
	topics       map[string]map[string]struct{} // topic -> set of client IDs
	nextID       atomic.Uint64
	keepAlive    time.Duration
	onConnect    func(string)
	onDisconnect func(string)
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
		topics:    make(map[string]map[string]struct{}),
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
		onConn := b.onConnect
		b.mu.Unlock()

		if onConn != nil {
			onConn(id)
		}

		defer func() {
			b.mu.Lock()
			delete(b.clients, id)
			for topic, subs := range b.topics {
				delete(subs, id)
				if len(subs) == 0 {
					delete(b.topics, topic)
				}
			}
			onDisc := b.onDisconnect
			b.mu.Unlock()

			if onDisc != nil {
				onDisc(id)
			}
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

// SendJSON marshals data to JSON and sends it as an SSE event to the
// specified client. It returns an error if JSON marshaling fails, or if
// the client was not found.
func (b *Broker) SendJSON(clientID string, event string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("sseutil: marshal json: %w", err)
	}
	ok := b.Send(clientID, Event{Event: event, Data: string(raw)})
	if !ok {
		return fmt.Errorf("sseutil: client %s not found or buffer full", clientID)
	}
	return nil
}

// BroadcastJSON marshals data to JSON and broadcasts it as an SSE event
// to all connected clients. It returns an error if JSON marshaling fails.
func (b *Broker) BroadcastJSON(event string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("sseutil: marshal json: %w", err)
	}
	b.Broadcast(Event{Event: event, Data: string(raw)})
	return nil
}

// Subscribe adds the given client to the specified topics. If the client
// is not connected, the subscription is still recorded and will take
// effect if the client connects with the same ID.
func (b *Broker) Subscribe(clientID string, topics ...string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, topic := range topics {
		subs, ok := b.topics[topic]
		if !ok {
			subs = make(map[string]struct{})
			b.topics[topic] = subs
		}
		subs[clientID] = struct{}{}
	}
}

// PublishTopic sends an event to all clients subscribed to the given topic.
// Events are dropped for clients whose send buffers are full.
func (b *Broker) PublishTopic(topic string, event Event) {
	data := event.Bytes()
	b.mu.RLock()
	defer b.mu.RUnlock()
	subs, ok := b.topics[topic]
	if !ok {
		return
	}
	for clientID := range subs {
		client, exists := b.clients[clientID]
		if !exists {
			continue
		}
		select {
		case client.events <- data:
		default:
		}
	}
}

// OnConnect sets a callback function that is invoked when a new client
// connects. The callback receives the client ID.
func (b *Broker) OnConnect(fn func(clientID string)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onConnect = fn
}

// OnDisconnect sets a callback function that is invoked when a client
// disconnects. The callback receives the client ID.
func (b *Broker) OnDisconnect(fn func(clientID string)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onDisconnect = fn
}
