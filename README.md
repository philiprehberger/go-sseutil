# go-sseutil

Server-Sent Events (SSE) utilities for Go. Server broker and client, zero dependencies.

## Installation

```bash
go get github.com/philiprehberger/go-sseutil
```

## Usage

### Server (Broker)

```go
package main

import (
	"net/http"
	"time"

	"github.com/philiprehberger/go-sseutil"
)

func main() {
	broker := sseutil.NewBroker(sseutil.WithKeepAlive(15 * time.Second))

	http.Handle("/events", broker.Handler())

	go func() {
		for {
			broker.Broadcast(sseutil.Event{
				Event: "tick",
				Data:  "hello",
			})
			time.Sleep(1 * time.Second)
		}
	}()

	http.ListenAndServe(":8080", nil)
}
```

### Client

```go
package main

import (
	"context"
	"fmt"

	"github.com/philiprehberger/go-sseutil"
)

func main() {
	ctx := context.Background()
	stream, err := sseutil.Connect(ctx, "http://localhost:8080/events")
	if err != nil {
		panic(err)
	}
	defer stream.Close()

	for event := range stream.Events() {
		fmt.Printf("Event: %s, Data: %s\n", event.Event, event.Data)
	}
}
```

### Event Builder

```go
event := sseutil.Event{
	ID:    "1",
	Event: "update",
	Data:  "payload data",
	Retry: 5000,
}

fmt.Print(event.String())
// Output:
// id: 1
// event: update
// data: payload data
// retry: 5000
```

## API

| Type / Function | Description |
|---|---|
| `Event` | SSE event with ID, Event, Data, and Retry fields |
| `Event.Bytes()` | Encode event to SSE wire format |
| `Event.String()` | String representation of SSE event |
| `Broker` | Manages SSE client connections |
| `NewBroker(opts...)` | Create a new broker with options |
| `WithKeepAlive(d)` | Set keep-alive interval (default 30s) |
| `Broker.Handler()` | HTTP handler for SSE endpoint |
| `Broker.Broadcast(e)` | Send event to all clients |
| `Broker.Send(id, e)` | Send event to a specific client |
| `Broker.ClientCount()` | Number of connected clients |
| `Connect(ctx, url)` | Connect to an SSE endpoint |
| `Stream.Events()` | Channel of received events |
| `Stream.Close()` | Close the connection |
| `Stream.LastEventID()` | Last received event ID |

## License

MIT
