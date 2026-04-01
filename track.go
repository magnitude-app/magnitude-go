package magnitude

import (
	"context"
	"time"
)

// Track records an analytics event. The optional context parameter can be used
// for cancellation; pass nil to use a default background context.
func (c *Client) Track(eventName string, distinctID string, props Properties, ctx ...context.Context) {
	if len(ctx) > 0 && ctx[0] != nil {
		select {
		case <-ctx[0].Done():
			return
		default:
		}
	}

	merged := make(map[string]interface{})

	// Super properties first (lowest priority).
	c.mu.Lock()
	for k, v := range c.superProps {
		merged[k] = v
	}
	c.mu.Unlock()

	// User properties override super properties.
	for k, v := range props {
		merged[k] = v
	}

	// Library metadata.
	merged["$lib"] = LibName
	merged["$lib_version"] = LibVersion

	e := Event{
		EventName:  eventName,
		DistinctID: distinctID,
		Token:      c.apiKey,
		Timestamp:  time.Now().UnixMilli(),
		InsertID:   newUUID(),
		Properties: merged,
	}

	c.enqueue(e)
}

// Identify associates a user with traits. It fires a special "$identify" event
// containing the traits under "$set". The optional context parameter can be
// used for cancellation.
func (c *Client) Identify(distinctID string, traits Traits, ctx ...context.Context) {
	if len(ctx) > 0 && ctx[0] != nil {
		select {
		case <-ctx[0].Done():
			return
		default:
		}
	}

	props := make(map[string]interface{})

	c.mu.Lock()
	for k, v := range c.superProps {
		props[k] = v
	}
	c.mu.Unlock()

	props["$lib"] = LibName
	props["$lib_version"] = LibVersion
	props["$user_id"] = distinctID
	props["$set"] = map[string]interface{}(traits)

	e := Event{
		EventName:  "$identify",
		DistinctID: distinctID,
		Token:      c.apiKey,
		Timestamp:  time.Now().UnixMilli(),
		InsertID:   newUUID(),
		Properties: props,
	}

	c.enqueue(e)
}

// Register adds super properties that are automatically merged into every
// subsequent event. Existing keys are overwritten.
func (c *Client) Register(props Properties) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range props {
		c.superProps[k] = v
	}
}
