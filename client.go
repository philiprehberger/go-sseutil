package sseutil

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// Stream is an SSE client that connects to an SSE endpoint and parses
// incoming events into a channel.
type Stream struct {
	mu          sync.Mutex
	url         string
	resp        *http.Response
	events      chan Event
	done        chan struct{}
	lastEventID string
	client      *http.Client
}

// Connect establishes a connection to the given SSE endpoint URL and
// returns a Stream that emits parsed events. The context controls the
// lifetime of the connection.
func Connect(ctx context.Context, url string) (*Stream, error) {
	s := &Stream{
		url:    url,
		events: make(chan Event, 256),
		done:   make(chan struct{}),
		client: http.DefaultClient,
	}

	if err := s.connect(ctx); err != nil {
		return nil, err
	}

	go s.readLoop(ctx)

	return s, nil
}

// connect performs the HTTP request to the SSE endpoint.
func (s *Stream) connect(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return fmt.Errorf("sseutil: creating request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	s.mu.Lock()
	lastID := s.lastEventID
	s.mu.Unlock()

	if lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("sseutil: connecting to %s: %w", s.url, err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("sseutil: unexpected status %d from %s", resp.StatusCode, s.url)
	}

	s.mu.Lock()
	s.resp = resp
	s.mu.Unlock()

	return nil
}

// readLoop reads and parses SSE events from the response body.
func (s *Stream) readLoop(ctx context.Context) {
	defer close(s.events)

	s.mu.Lock()
	resp := s.resp
	s.mu.Unlock()

	scanner := bufio.NewScanner(resp.Body)
	var (
		id    string
		event string
		data  strings.Builder
		hasData bool
	)

	for scanner.Scan() {
		select {
		case <-s.done:
			return
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()

		// Empty line: dispatch event.
		if line == "" {
			if hasData {
				d := data.String()
				// Remove trailing newline added during accumulation.
				if len(d) > 0 && d[len(d)-1] == '\n' {
					d = d[:len(d)-1]
				}

				evt := Event{
					ID:    id,
					Event: event,
					Data:  d,
				}

				if id != "" {
					s.mu.Lock()
					s.lastEventID = id
					s.mu.Unlock()
				}

				select {
				case s.events <- evt:
				case <-s.done:
					return
				case <-ctx.Done():
					return
				}
			}
			// Reset fields.
			id = ""
			event = ""
			data.Reset()
			hasData = false
			continue
		}

		// Comment line.
		if strings.HasPrefix(line, ":") {
			continue
		}

		// Parse field.
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")

		switch field {
		case "id":
			id = value
		case "event":
			event = value
		case "data":
			hasData = true
			data.WriteString(value)
			data.WriteByte('\n')
		case "retry":
			// Retry is informational for the client; we parse but
			// don't act on it in this implementation.
			if _, err := strconv.Atoi(value); err == nil {
				// Valid retry value; could be stored if needed.
			}
		}
	}
}

// Events returns a receive-only channel of parsed SSE events.
// The channel is closed when the stream ends or Close is called.
func (s *Stream) Events() <-chan Event {
	return s.events
}

// Close closes the SSE stream and releases resources.
func (s *Stream) Close() error {
	select {
	case <-s.done:
		return nil
	default:
		close(s.done)
	}

	s.mu.Lock()
	resp := s.resp
	s.mu.Unlock()

	if resp != nil && resp.Body != nil {
		return resp.Body.Close()
	}
	return nil
}

// LastEventID returns the ID of the last received event.
func (s *Stream) LastEventID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastEventID
}
