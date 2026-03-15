// Package sseutil provides Server-Sent Events utilities for Go.
//
// It includes a server-side broker for managing SSE client connections and
// broadcasting events, a client for consuming SSE streams, and an event
// builder for constructing SSE-formatted messages.
package sseutil

import (
	"fmt"
	"strings"
)

// Event represents a Server-Sent Event with standard fields.
type Event struct {
	// ID is the event identifier. If non-empty, it is sent as the "id" field.
	ID string

	// Event is the event type. If non-empty, it is sent as the "event" field.
	Event string

	// Data is the event payload. It is always included in the SSE output.
	// Multi-line data is split so each line is prefixed with "data: ".
	Data string

	// Retry is the reconnection interval in milliseconds.
	// If zero, the retry field is omitted from the output.
	Retry int
}

// Bytes encodes the event into SSE wire format.
//
// Only non-empty fields are included, except Data which is always present.
// Multi-line data values are split on newline characters, with each line
// emitted as a separate "data:" field.
func (e Event) Bytes() []byte {
	var b strings.Builder

	if e.ID != "" {
		fmt.Fprintf(&b, "id: %s\n", e.ID)
	}

	if e.Event != "" {
		fmt.Fprintf(&b, "event: %s\n", e.Event)
	}

	lines := strings.Split(e.Data, "\n")
	for _, line := range lines {
		fmt.Fprintf(&b, "data: %s\n", line)
	}

	if e.Retry > 0 {
		fmt.Fprintf(&b, "retry: %d\n", e.Retry)
	}

	b.WriteString("\n")

	return []byte(b.String())
}

// String returns the SSE-formatted event as a string.
func (e Event) String() string {
	return string(e.Bytes())
}
