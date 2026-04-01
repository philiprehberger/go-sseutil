# go-sseutil

[![CI](https://github.com/philiprehberger/go-sseutil/actions/workflows/ci.yml/badge.svg)](https://github.com/philiprehberger/go-sseutil/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/philiprehberger/go-sseutil.svg)](https://pkg.go.dev/github.com/philiprehberger/go-sseutil)
[![Last updated](https://img.shields.io/github/last-commit/philiprehberger/go-sseutil)](https://github.com/philiprehberger/go-sseutil/commits/main)

Server-Sent Events (SSE) utilities for Go. Server broker and client, zero dependencies

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

### JSON Events

```go
// Send JSON to a specific client.
type Message struct {
	Text string `json:"text"`
	From string `json:"from"`
}

err := broker.SendJSON("client-1", "chat", Message{Text: "hi", From: "alice"})
// Sends: event: chat\ndata: {"text":"hi","from":"alice"}\n\n

// Broadcast JSON to all clients.
err = broker.BroadcastJSON("update", map[string]any{"status": "ok"})
```

### Topics

```go
// Subscribe a client to topics.
broker.Subscribe("client-1", "sports", "news")

// Publish to a topic — only subscribed clients receive the event.
broker.PublishTopic("sports", sseutil.Event{Event: "score", Data: "3-2"})
```

### Lifecycle Hooks

```go
broker.OnConnect(func(clientID string) {
	log.Printf("client connected: %s", clientID)
})

broker.OnDisconnect(func(clientID string) {
	log.Printf("client disconnected: %s", clientID)
})
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
| `Broker.SendJSON(id, event, data)` | Marshal data to JSON and send to a client |
| `Broker.BroadcastJSON(event, data)` | Marshal data to JSON and broadcast to all |
| `Broker.Subscribe(id, topics...)` | Subscribe a client to named topics |
| `Broker.PublishTopic(topic, e)` | Send event to all clients subscribed to topic |
| `Broker.OnConnect(fn)` | Set callback for client connections |
| `Broker.OnDisconnect(fn)` | Set callback for client disconnections |
| `Broker.ClientCount()` | Number of connected clients |
| `Connect(ctx, url)` | Connect to an SSE endpoint |
| `Stream.Events()` | Channel of received events |
| `Stream.Close()` | Close the connection |
| `Stream.LastEventID()` | Last received event ID |

## Development

```bash
go test ./...
go vet ./...
```

## Support

If you find this project useful:

⭐ [Star the repo](https://github.com/philiprehberger/go-sseutil)

🐛 [Report issues](https://github.com/philiprehberger/go-sseutil/issues?q=is%3Aissue+is%3Aopen+label%3Abug)

💡 [Suggest features](https://github.com/philiprehberger/go-sseutil/issues?q=is%3Aissue+is%3Aopen+label%3Aenhancement)

❤️ [Sponsor development](https://github.com/sponsors/philiprehberger)

🌐 [All Open Source Projects](https://philiprehberger.com/open-source-packages)

💻 [GitHub Profile](https://github.com/philiprehberger)

🔗 [LinkedIn Profile](https://www.linkedin.com/in/philiprehberger)

## License

[MIT](LICENSE)
