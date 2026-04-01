# magnitude-go

Go SDK for [Magnitude](https://magnitude.so) analytics. Zero dependencies — pure stdlib.

## Install

```bash
go get github.com/mehulkapadia5/magnitude-go
```

## Quick Start

```go
package main

import (
    "time"
    magnitude "github.com/mehulkapadia5/magnitude-go"
)

func main() {
    client := magnitude.NewClient("YOUR_API_KEY", &magnitude.Config{
        BatchSize:     200,
        FlushInterval: 5 * time.Second,
    })
    defer client.Close()

    // Track an event
    client.Track("page_viewed", "user_123", magnitude.Properties{
        "page": "/dashboard",
    })

    // Identify a user
    client.Identify("user_123", magnitude.Traits{
        "email": "jane@example.com",
        "name":  "Jane Doe",
        "plan":  "pro",
    })

    // Set super properties (attached to every event)
    client.Register(magnitude.Properties{
        "app_version": "2.1.0",
        "platform":    "backend",
    })
}
```

## Configuration

| Option | Default | Description |
|---|---|---|
| `ApiUrl` | Magnitude hosted endpoint | Ingest endpoint URL |
| `BatchSize` | `200` | Max events buffered before auto-flush |
| `FlushInterval` | `5s` | Background flush interval |
| `HTTPClient` | `http.DefaultClient` | Custom HTTP client |
| `OnError` | `nil` | Callback for batch send failures |

## Features

- **Thread-safe** — safe for concurrent use from multiple goroutines
- **Automatic batching** — buffers events, flushes on interval or batch size
- **Retry with backoff** — 3 retries with exponential backoff on 429/5xx
- **Graceful shutdown** — `Close()` flushes remaining events
- **Super properties** — `Register()` attaches properties to every event
- **Deduplication** — every event gets a UUID `$insert_id`
- **Context support** — optional `context.Context` for cancellation
- **Zero dependencies** — pure Go stdlib

## API

### `NewClient(apiKey string, config *Config) *Client`

Creates a new client. Pass `nil` for config to use defaults.

### `client.Track(eventName, distinctID string, props Properties, ctx ...context.Context)`

Track an analytics event.

### `client.Identify(distinctID string, traits Traits, ctx ...context.Context)`

Identify a user with traits. Fires a `$identify` event.

### `client.Register(props Properties)`

Set super properties merged into every subsequent event.

### `client.Flush()`

Manually flush all buffered events.

### `client.Close()`

Flush remaining events and stop the background flusher. Always call this before your program exits.

## License

MIT
