package sseutil

import (
	"testing"
)

func TestEvent_Bytes(t *testing.T) {
	e := Event{
		ID:    "1",
		Event: "message",
		Data:  "hello",
		Retry: 5000,
	}

	got := string(e.Bytes())
	want := "id: 1\nevent: message\ndata: hello\nretry: 5000\n\n"

	if got != want {
		t.Errorf("Event.Bytes() =\n%q\nwant\n%q", got, want)
	}
}

func TestEvent_Bytes_MultilineData(t *testing.T) {
	e := Event{
		Data: "line1\nline2\nline3",
	}

	got := string(e.Bytes())
	want := "data: line1\ndata: line2\ndata: line3\n\n"

	if got != want {
		t.Errorf("Event.Bytes() multiline =\n%q\nwant\n%q", got, want)
	}
}

func TestEvent_Bytes_OnlyData(t *testing.T) {
	e := Event{
		Data: "just data",
	}

	got := string(e.Bytes())
	want := "data: just data\n\n"

	if got != want {
		t.Errorf("Event.Bytes() only data =\n%q\nwant\n%q", got, want)
	}
}

func TestEvent_String(t *testing.T) {
	e := Event{
		ID:   "42",
		Data: "test",
	}

	got := e.String()
	want := "id: 42\ndata: test\n\n"

	if got != want {
		t.Errorf("Event.String() =\n%q\nwant\n%q", got, want)
	}
}
