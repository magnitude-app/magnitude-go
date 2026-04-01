// Package magnitude provides a Go SDK for the Magnitude analytics platform.
//
// It supports event tracking, user identification, and super properties with
// automatic batching, retry with backoff, and graceful shutdown.
package magnitude

import (
	"net/http"
	"sync"
	"time"
)

const (
	// LibName is the library identifier sent with every event.
	LibName = "magnitude-go"
	// LibVersion is the current SDK version.
	LibVersion = "1.0.0"

	defaultAPIURL        = "https://zhapiidqiqmsphqocuei.supabase.co/functions/v1/ingest-events"
	defaultBatchSize     = 200
	defaultFlushInterval = 5 * time.Second
)

// Config holds optional configuration for the Magnitude client.
type Config struct {
	// ApiUrl is the ingest endpoint URL.
	// Default: the Magnitude hosted endpoint.
	ApiUrl string

	// BatchSize is the maximum number of events buffered before an
	// automatic flush. Default: 200.
	BatchSize int

	// FlushInterval controls how often the background flusher runs.
	// Default: 5 seconds.
	FlushInterval time.Duration

	// HTTPClient allows providing a custom *http.Client. Default: http.DefaultClient.
	HTTPClient *http.Client

	// OnError is an optional callback invoked when a batch fails after all retries.
	OnError func(error)
}

// Client is a thread-safe Magnitude analytics client.
type Client struct {
	apiKey     string
	config     Config
	httpClient *http.Client

	mu         sync.Mutex
	queue      []Event
	superProps map[string]interface{}

	ticker *time.Ticker
	done   chan struct{}
}

// NewClient creates a new Magnitude client. Pass nil for config to use defaults.
func NewClient(apiKey string, config *Config) *Client {
	cfg := Config{
		ApiUrl:        defaultAPIURL,
		BatchSize:     defaultBatchSize,
		FlushInterval: defaultFlushInterval,
	}

	if config != nil {
		if config.ApiUrl != "" {
			cfg.ApiUrl = config.ApiUrl
		}
		if config.BatchSize > 0 {
			cfg.BatchSize = config.BatchSize
		}
		if config.FlushInterval > 0 {
			cfg.FlushInterval = config.FlushInterval
		}
		if config.HTTPClient != nil {
			cfg.HTTPClient = config.HTTPClient
		}
		if config.OnError != nil {
			cfg.OnError = config.OnError
		}
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	c := &Client{
		apiKey:     apiKey,
		config:     cfg,
		httpClient: httpClient,
		superProps: make(map[string]interface{}),
	}

	c.startFlusher()
	return c
}

// Close flushes any remaining events and stops the background flusher.
// It should be called when the client is no longer needed.
func (c *Client) Close() {
	if c.ticker != nil {
		c.ticker.Stop()
	}
	if c.done != nil {
		close(c.done)
	}
	c.Flush()
}
