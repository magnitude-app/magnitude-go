package magnitude

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTrackSendsEvent(t *testing.T) {
	var received eventPayload
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer server.Close()

	client := NewClient("test-key", &Config{
		ApiUrl:        server.URL,
		BatchSize:     10,
		FlushInterval: 1 * time.Hour, // don't auto-flush
	})

	client.Track("page_view", "user_1", Properties{"page": "/home"})
	client.Flush()
	client.Close()

	mu.Lock()
	defer mu.Unlock()

	if received.APIKey != "test-key" {
		t.Errorf("expected api_key 'test-key', got %q", received.APIKey)
	}
	if len(received.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received.Events))
	}

	e := received.Events[0]
	if e.EventName != "page_view" {
		t.Errorf("expected event 'page_view', got %q", e.EventName)
	}
	if e.DistinctID != "user_1" {
		t.Errorf("expected distinct_id 'user_1', got %q", e.DistinctID)
	}
	if e.Token != "test-key" {
		t.Errorf("expected token 'test-key', got %q", e.Token)
	}
	if e.InsertID == "" {
		t.Error("expected $insert_id to be set")
	}
	if e.Timestamp == 0 {
		t.Error("expected timestamp to be set")
	}
	if e.Properties["page"] != "/home" {
		t.Errorf("expected page '/home', got %v", e.Properties["page"])
	}
	if e.Properties["$lib"] != LibName {
		t.Errorf("expected $lib %q, got %v", LibName, e.Properties["$lib"])
	}
	if e.Properties["$lib_version"] != LibVersion {
		t.Errorf("expected $lib_version %q, got %v", LibVersion, e.Properties["$lib_version"])
	}
}

func TestIdentifySendsIdentifyEvent(t *testing.T) {
	var received eventPayload
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer server.Close()

	client := NewClient("test-key", &Config{
		ApiUrl:        server.URL,
		BatchSize:     10,
		FlushInterval: 1 * time.Hour,
	})

	client.Identify("user_1", Traits{"email": "test@example.com", "plan": "pro"})
	client.Flush()
	client.Close()

	mu.Lock()
	defer mu.Unlock()

	if len(received.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received.Events))
	}

	e := received.Events[0]
	if e.EventName != "$identify" {
		t.Errorf("expected event '$identify', got %q", e.EventName)
	}
	if e.Properties["$user_id"] != "user_1" {
		t.Errorf("expected $user_id 'user_1', got %v", e.Properties["$user_id"])
	}

	setMap, ok := e.Properties["$set"].(map[string]interface{})
	if !ok {
		t.Fatal("expected $set to be a map")
	}
	if setMap["email"] != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %v", setMap["email"])
	}
}

func TestSuperProperties(t *testing.T) {
	var received eventPayload
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer server.Close()

	client := NewClient("test-key", &Config{
		ApiUrl:        server.URL,
		BatchSize:     10,
		FlushInterval: 1 * time.Hour,
	})

	client.Register(Properties{"app_version": "2.0", "platform": "backend"})
	client.Track("test_event", "user_1", Properties{"custom": "value"})
	client.Flush()
	client.Close()

	mu.Lock()
	defer mu.Unlock()

	e := received.Events[0]
	if e.Properties["app_version"] != "2.0" {
		t.Errorf("expected super prop app_version '2.0', got %v", e.Properties["app_version"])
	}
	if e.Properties["platform"] != "backend" {
		t.Errorf("expected super prop platform 'backend', got %v", e.Properties["platform"])
	}
	if e.Properties["custom"] != "value" {
		t.Errorf("expected custom 'value', got %v", e.Properties["custom"])
	}
}

func TestBatchFlushOnSize(t *testing.T) {
	var callCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(200)
	}))
	defer server.Close()

	client := NewClient("test-key", &Config{
		ApiUrl:        server.URL,
		BatchSize:     3,
		FlushInterval: 1 * time.Hour,
	})

	client.Track("e1", "u1", nil)
	client.Track("e2", "u1", nil)
	client.Track("e3", "u1", nil) // should trigger flush

	// Give the flush a moment to complete.
	time.Sleep(100 * time.Millisecond)
	client.Close()

	if c := atomic.LoadInt32(&callCount); c < 1 {
		t.Errorf("expected at least 1 flush call, got %d", c)
	}
}

func TestRetryOn5xx(t *testing.T) {
	var callCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n < 3 {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
	}))
	defer server.Close()

	client := NewClient("test-key", &Config{
		ApiUrl:        server.URL,
		BatchSize:     10,
		FlushInterval: 1 * time.Hour,
	})

	client.Track("retry_event", "u1", nil)
	client.Flush()
	client.Close()

	if c := atomic.LoadInt32(&callCount); c != 3 {
		t.Errorf("expected 3 attempts, got %d", c)
	}
}

func TestDropOn400(t *testing.T) {
	var callCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(400)
	}))
	defer server.Close()

	client := NewClient("test-key", &Config{
		ApiUrl:        server.URL,
		BatchSize:     10,
		FlushInterval: 1 * time.Hour,
	})

	client.Track("bad_event", "u1", nil)
	client.Flush()
	client.Close()

	if c := atomic.LoadInt32(&callCount); c != 1 {
		t.Errorf("expected 1 attempt (no retry on 400), got %d", c)
	}
}

func TestConcurrentTrack(t *testing.T) {
	var mu sync.Mutex
	var totalEvents int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload eventPayload
		json.NewDecoder(r.Body).Decode(&payload)
		mu.Lock()
		totalEvents += len(payload.Events)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer server.Close()

	client := NewClient("test-key", &Config{
		ApiUrl:        server.URL,
		BatchSize:     50,
		FlushInterval: 1 * time.Hour,
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			client.Track("concurrent_event", "user_1", Properties{"n": n})
		}(i)
	}
	wg.Wait()
	client.Close()

	mu.Lock()
	defer mu.Unlock()
	if totalEvents != 100 {
		t.Errorf("expected 100 events, got %d", totalEvents)
	}
}

func TestUUIDFormat(t *testing.T) {
	id := newUUID()
	if len(id) != 36 {
		t.Errorf("expected UUID length 36, got %d: %q", len(id), id)
	}
	// Check dashes at correct positions.
	if id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		t.Errorf("UUID has wrong format: %q", id)
	}
}

func TestFlushIntervalTrigger(t *testing.T) {
	var callCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(200)
	}))
	defer server.Close()

	client := NewClient("test-key", &Config{
		ApiUrl:        server.URL,
		BatchSize:     1000, // high so it won't trigger by size
		FlushInterval: 100 * time.Millisecond,
	})

	client.Track("timed_event", "u1", nil)
	time.Sleep(300 * time.Millisecond)
	client.Close()

	if c := atomic.LoadInt32(&callCount); c < 1 {
		t.Errorf("expected flush via interval, got %d calls", c)
	}
}

func TestPayloadFormat(t *testing.T) {
	var rawBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		r.Body.Read(buf)
		rawBody = buf
		w.WriteHeader(200)
	}))
	defer server.Close()

	client := NewClient("my-api-key", &Config{
		ApiUrl:        server.URL,
		BatchSize:     10,
		FlushInterval: 1 * time.Hour,
	})

	client.Track("test", "u1", Properties{"key": "val"})
	client.Flush()
	client.Close()

	var parsed map[string]interface{}
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		t.Fatalf("failed to parse payload: %v", err)
	}

	if parsed["api_key"] != "my-api-key" {
		t.Errorf("expected api_key 'my-api-key', got %v", parsed["api_key"])
	}

	events, ok := parsed["events"].([]interface{})
	if !ok || len(events) == 0 {
		t.Fatal("expected events array")
	}

	event := events[0].(map[string]interface{})
	requiredFields := []string{"event", "distinct_id", "token", "timestamp", "$insert_id", "properties"}
	for _, f := range requiredFields {
		if _, exists := event[f]; !exists {
			t.Errorf("missing required field %q in event", f)
		}
	}
}
