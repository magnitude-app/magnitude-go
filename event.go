package magnitude

// Properties is a map of event properties.
type Properties map[string]interface{}

// Traits is a map of user traits for identification.
type Traits map[string]interface{}

// Event represents a single analytics event to be sent to the ingest endpoint.
type Event struct {
	EventName  string                 `json:"event"`
	DistinctID string                 `json:"distinct_id"`
	Token      string                 `json:"token"`
	Timestamp  int64                  `json:"timestamp"`
	InsertID   string                 `json:"$insert_id"`
	Properties map[string]interface{} `json:"properties"`
}

// eventPayload is the top-level JSON body sent to the ingest endpoint.
type eventPayload struct {
	APIKey string  `json:"api_key"`
	Events []Event `json:"events"`
}
