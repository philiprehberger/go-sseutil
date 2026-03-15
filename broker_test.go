package sseutil

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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
