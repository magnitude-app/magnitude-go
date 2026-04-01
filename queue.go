package magnitude

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	maxRetries     = 3
	initialBackoff = 1 * time.Second
)

// enqueue adds an event to the internal buffer. If the buffer reaches
// BatchSize, a flush is triggered automatically.
func (c *Client) enqueue(e Event) {
	c.mu.Lock()
	c.queue = append(c.queue, e)
	shouldFlush := len(c.queue) >= c.config.BatchSize
	c.mu.Unlock()

	if shouldFlush {
		c.Flush()
	}
}

// startFlusher launches a background goroutine that flushes the queue
// on every FlushInterval tick.
func (c *Client) startFlusher() {
	c.ticker = time.NewTicker(c.config.FlushInterval)
	c.done = make(chan struct{})

	go func() {
		for {
			select {
			case <-c.ticker.C:
				c.Flush()
			case <-c.done:
				return
			}
		}
	}()
}

// Flush sends all buffered events to the ingest endpoint.
func (c *Client) Flush() {
	c.mu.Lock()
	if len(c.queue) == 0 {
		c.mu.Unlock()
		return
	}
	batch := c.queue
	c.queue = nil
	c.mu.Unlock()

	c.send(batch)
}

// send posts a batch of events with retry logic.
func (c *Client) send(batch []Event) {
	payload := eventPayload{
		APIKey: c.apiKey,
		Events: batch,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	backoff := initialBackoff

	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequest(http.MethodPost, c.config.ApiUrl, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return // success
		}

		// Drop batch on 400 (bad request) — no point retrying malformed data.
		if resp.StatusCode == 400 {
			return
		}

		// Retry on 429 (rate limit) or 5xx (server error).
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			time.Sleep(backoff)
			backoff *= 2
			continue
		}

		// Any other status — don't retry.
		return
	}

	// All retries exhausted — drop the batch.
	if c.config.OnError != nil {
		c.config.OnError(fmt.Errorf("magnitude: failed to send batch of %d events after %d retries", len(batch), maxRetries))
	}
}
