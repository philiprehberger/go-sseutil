package sseutil

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewBroker(t *testing.T) {
	b := NewBroker()
	if b == nil {
		t.Fatal("NewBroker() returned nil")
	}
	if b.keepAlive != 30*time.Second {
		t.Errorf("default keepAlive = %v, want 30s", b.keepAlive)
	}

	b2 := NewBroker(WithKeepAlive(10 * time.Second))
	if b2.keepAlive != 10*time.Second {
		t.Errorf("keepAlive = %v, want 10s", b2.keepAlive)
	}
}

func TestBroker_Handler(t *testing.T) {
	b := NewBroker(WithKeepAlive(0))
	server := httptest.NewServer(b.Handler())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}
}

func TestBroker_Broadcast(t *testing.T) {
	b := NewBroker(WithKeepAlive(0))
	server := httptest.NewServer(b.Handler())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Wait for client to be registered.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if b.ClientCount() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	b.Broadcast(Event{ID: "1", Data: "hello"})

	scanner := bufio.NewScanner(resp.Body)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
		// After seeing the empty line (event boundary), stop.
		if line == "" && len(lines) > 1 {
			break
		}
	}

	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "data: hello") {
		t.Errorf("broadcast event not received, got:\n%s", joined)
	}
}

func TestBroker_ClientCount(t *testing.T) {
	b := NewBroker(WithKeepAlive(0))
	server := httptest.NewServer(b.Handler())
	defer server.Close()

	if b.ClientCount() != 0 {
		t.Fatalf("initial ClientCount = %d, want 0", b.ClientCount())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if b.ClientCount() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if b.ClientCount() != 1 {
		t.Errorf("ClientCount after connect = %d, want 1", b.ClientCount())
	}

	cancel()

	deadline = time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if b.ClientCount() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if b.ClientCount() != 0 {
		t.Errorf("ClientCount after disconnect = %d, want 0", b.ClientCount())
	}
}

func TestBroker_ClientDisconnect(t *testing.T) {
	b := NewBroker(WithKeepAlive(0))
	server := httptest.NewServer(b.Handler())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if b.ClientCount() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if b.ClientCount() != 1 {
		t.Fatalf("ClientCount = %d, want 1", b.ClientCount())
	}

	// Cancel context to simulate disconnect.
	cancel()

	deadline = time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if b.ClientCount() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if b.ClientCount() != 0 {
		t.Errorf("ClientCount after cancel = %d, want 0", b.ClientCount())
	}
}

// waitForClients polls until the broker has the expected number of clients.
func waitForClients(t *testing.T, b *Broker, count int) {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if b.ClientCount() == count {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d clients, have %d", count, b.ClientCount())
}

func TestBroker_SendJSON(t *testing.T) {
	b := NewBroker(WithKeepAlive(0))
	server := httptest.NewServer(b.Handler())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	waitForClients(t, b, 1)

	// Get the client ID.
	b.mu.RLock()
	var clientID string
	for id := range b.clients {
		clientID = id
	}
	b.mu.RUnlock()

	type payload struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	err = b.SendJSON(clientID, "update", payload{Name: "test", Value: 42})
	if err != nil {
		t.Fatalf("SendJSON() error: %v", err)
	}

	scanner := bufio.NewScanner(resp.Body)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
		if line == "" && len(lines) > 1 {
			break
		}
	}

	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "event: update") {
		t.Errorf("expected event type 'update', got:\n%s", joined)
	}

	// Verify JSON data is present and valid.
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			dataStr := strings.TrimPrefix(line, "data: ")
			var p payload
			if err := json.Unmarshal([]byte(dataStr), &p); err != nil {
				t.Errorf("data is not valid JSON: %v", err)
			}
			if p.Name != "test" || p.Value != 42 {
				t.Errorf("JSON payload = %+v, want {test 42}", p)
			}
		}
	}
}

func TestBroker_SendJSON_MarshalError(t *testing.T) {
	b := NewBroker()
	// Channels cannot be marshaled to JSON.
	err := b.SendJSON("nonexistent", "test", make(chan int))
	if err == nil {
		t.Fatal("SendJSON() with unmarshalable data should return error")
	}
}

func TestBroker_SendJSON_ClientNotFound(t *testing.T) {
	b := NewBroker()
	err := b.SendJSON("nonexistent", "test", "hello")
	if err == nil {
		t.Fatal("SendJSON() to nonexistent client should return error")
	}
}

func TestBroker_BroadcastJSON(t *testing.T) {
	b := NewBroker(WithKeepAlive(0))
	server := httptest.NewServer(b.Handler())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	waitForClients(t, b, 1)

	data := map[string]string{"msg": "hello"}
	err = b.BroadcastJSON("notification", data)
	if err != nil {
		t.Fatalf("BroadcastJSON() error: %v", err)
	}

	scanner := bufio.NewScanner(resp.Body)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
		if line == "" && len(lines) > 1 {
			break
		}
	}

	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, `"msg":"hello"`) {
		t.Errorf("BroadcastJSON data not received, got:\n%s", joined)
	}
}

func TestBroker_BroadcastJSON_MarshalError(t *testing.T) {
	b := NewBroker()
	err := b.BroadcastJSON("test", make(chan int))
	if err == nil {
		t.Fatal("BroadcastJSON() with unmarshalable data should return error")
	}
}

func TestBroker_Subscribe_PublishTopic(t *testing.T) {
	b := NewBroker(WithKeepAlive(0))
	server := httptest.NewServer(b.Handler())
	defer server.Close()

	// Connect two clients.
	ctx1, cancel1 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel1()
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()

	req1, _ := http.NewRequestWithContext(ctx1, http.MethodGet, server.URL, nil)
	resp1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp1.Body.Close()

	req2, _ := http.NewRequestWithContext(ctx2, http.MethodGet, server.URL, nil)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	waitForClients(t, b, 2)

	// Get both client IDs.
	b.mu.RLock()
	var ids []string
	for id := range b.clients {
		ids = append(ids, id)
	}
	b.mu.RUnlock()

	// Subscribe only the first client to "news".
	b.Subscribe(ids[0], "news")

	// Publish to "news" topic.
	b.PublishTopic("news", Event{Event: "news", Data: "breaking"})

	// Also broadcast to identify which response belongs to which client.
	// The subscribed client should receive "breaking", the other should not
	// (we verify by sending a marker event to all and checking order).

	// Send a marker so both clients have something to read.
	b.Broadcast(Event{Event: "marker", Data: "end"})

	// Read from resp1 and resp2.
	readEvents := func(resp *http.Response) []string {
		scanner := bufio.NewScanner(resp.Body)
		var collected []string
		for scanner.Scan() {
			line := scanner.Text()
			collected = append(collected, line)
			// Stop after we see the marker event boundary.
			if strings.HasPrefix(line, "data: end") {
				// Read the trailing empty line.
				scanner.Scan()
				break
			}
		}
		return collected
	}

	lines1 := readEvents(resp1)
	lines2 := readEvents(resp2)

	joined1 := strings.Join(lines1, "\n")
	joined2 := strings.Join(lines2, "\n")

	// One of them should have "breaking", the other should not.
	has1 := strings.Contains(joined1, "data: breaking")
	has2 := strings.Contains(joined2, "data: breaking")

	if !has1 && !has2 {
		t.Error("neither client received the topic event")
	}
	if has1 && has2 {
		t.Error("both clients received the topic event; only the subscribed one should")
	}
}

func TestBroker_PublishTopic_NoSubscribers(t *testing.T) {
	b := NewBroker(WithKeepAlive(0))
	// Should not panic with no subscribers.
	b.PublishTopic("empty-topic", Event{Data: "test"})
}

func TestBroker_Subscribe_MultipleTopics(t *testing.T) {
	b := NewBroker()
	b.Subscribe("client-1", "sports", "news", "weather")

	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, topic := range []string{"sports", "news", "weather"} {
		subs, ok := b.topics[topic]
		if !ok {
			t.Errorf("topic %q not found", topic)
			continue
		}
		if _, exists := subs["client-1"]; !exists {
			t.Errorf("client-1 not subscribed to %q", topic)
		}
	}
}

func TestBroker_OnConnect(t *testing.T) {
	b := NewBroker(WithKeepAlive(0))

	var mu sync.Mutex
	var connected []string
	b.OnConnect(func(id string) {
		mu.Lock()
		connected = append(connected, id)
		mu.Unlock()
	})

	server := httptest.NewServer(b.Handler())
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	waitForClients(t, b, 1)

	mu.Lock()
	count := len(connected)
	mu.Unlock()

	if count != 1 {
		t.Errorf("OnConnect called %d times, want 1", count)
	}
}

func TestBroker_OnDisconnect(t *testing.T) {
	b := NewBroker(WithKeepAlive(0))

	var mu sync.Mutex
	var disconnected []string
	b.OnDisconnect(func(id string) {
		mu.Lock()
		disconnected = append(disconnected, id)
		mu.Unlock()
	})

	server := httptest.NewServer(b.Handler())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	waitForClients(t, b, 1)

	cancel()

	waitForClients(t, b, 0)

	mu.Lock()
	count := len(disconnected)
	mu.Unlock()

	if count != 1 {
		t.Errorf("OnDisconnect called %d times, want 1", count)
	}
}

func TestBroker_OnDisconnect_CleansTopics(t *testing.T) {
	b := NewBroker(WithKeepAlive(0))

	var connectedID string
	var mu sync.Mutex
	b.OnConnect(func(id string) {
		mu.Lock()
		connectedID = id
		mu.Unlock()
	})

	server := httptest.NewServer(b.Handler())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	waitForClients(t, b, 1)

	mu.Lock()
	id := connectedID
	mu.Unlock()

	b.Subscribe(id, "alerts")

	// Verify subscription exists.
	b.mu.RLock()
	_, hasTopic := b.topics["alerts"]
	b.mu.RUnlock()
	if !hasTopic {
		t.Fatal("topic 'alerts' should exist after subscribe")
	}

	cancel()
	waitForClients(t, b, 0)

	// Topic should be cleaned up since the only subscriber disconnected.
	b.mu.RLock()
	_, hasTopic = b.topics["alerts"]
	b.mu.RUnlock()
	if hasTopic {
		t.Error("topic 'alerts' should be removed after last subscriber disconnects")
	}
}
