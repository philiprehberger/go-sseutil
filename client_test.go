package sseutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestSSEServer creates a test HTTP server that sends SSE events.
func newTestSSEServer(events []Event) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flusher", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		for _, e := range events {
			_, err := w.Write(e.Bytes())
			if err != nil {
				return
			}
			flusher.Flush()
		}

		// Keep connection open briefly so client can read.
		<-r.Context().Done()
	}))
}

func TestConnect(t *testing.T) {
	server := newTestSSEServer([]Event{
		{ID: "1", Data: "hello"},
	})
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stream, err := Connect(ctx, server.URL)
	if err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer stream.Close()

	if stream == nil {
		t.Fatal("Connect() returned nil stream")
	}
}

func TestStream_Events(t *testing.T) {
	events := []Event{
		{ID: "1", Event: "msg", Data: "first"},
		{ID: "2", Event: "msg", Data: "second"},
		{ID: "3", Data: "third"},
	}

	server := newTestSSEServer(events)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stream, err := Connect(ctx, server.URL)
	if err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer stream.Close()

	var received []Event
	for i := 0; i < len(events); i++ {
		select {
		case evt, ok := <-stream.Events():
			if !ok {
				t.Fatalf("events channel closed after %d events", i)
			}
			received = append(received, evt)
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for event %d", i)
		}
	}

	for i, want := range events {
		got := received[i]
		if got.ID != want.ID {
			t.Errorf("event[%d].ID = %q, want %q", i, got.ID, want.ID)
		}
		if got.Event != want.Event {
			t.Errorf("event[%d].Event = %q, want %q", i, got.Event, want.Event)
		}
		if got.Data != want.Data {
			t.Errorf("event[%d].Data = %q, want %q", i, got.Data, want.Data)
		}
	}

	lastID := stream.LastEventID()
	if lastID != "3" {
		t.Errorf("LastEventID() = %q, want %q", lastID, "3")
	}
}

func TestStream_Close(t *testing.T) {
	// Server that sends events slowly.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flusher", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		for i := 0; ; i++ {
			evt := Event{ID: fmt.Sprintf("%d", i), Data: "tick"}
			_, err := w.Write(evt.Bytes())
			if err != nil {
				return
			}
			flusher.Flush()
			time.Sleep(100 * time.Millisecond)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := Connect(ctx, server.URL)
	if err != nil {
		t.Fatalf("Connect() error: %v", err)
	}

	// Read at least one event.
	select {
	case _, ok := <-stream.Events():
		if !ok {
			t.Fatal("events channel closed before receiving any events")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first event")
	}

	// Close should stop the stream.
	if err := stream.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}

	// Events channel should eventually be closed.
	timer := time.NewTimer(1 * time.Second)
	defer timer.Stop()
	for {
		select {
		case _, ok := <-stream.Events():
			if !ok {
				return // Success: channel closed.
			}
			// May receive buffered events, keep draining.
		case <-timer.C:
			t.Error("events channel not closed after Close()")
			return
		}
	}
}
