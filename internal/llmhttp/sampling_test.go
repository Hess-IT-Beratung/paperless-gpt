package llmhttp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeAnthropic records every request body and, when rejectTemperature is set,
// answers like current Claude models do when a request carries a temperature.
type fakeAnthropic struct {
	rejectTemperature bool

	mu     sync.Mutex
	bodies []map[string]any
}

func (f *fakeAnthropic) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	f.mu.Lock()
	f.bodies = append(f.bodies, body)
	f.mu.Unlock()

	if _, ok := body["temperature"]; ok && f.rejectTemperature {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"type":"error","error":{"type":"invalid_request_error","message":"`+"`temperature`"+` is deprecated for this model."}}`)
		return
	}
	_, _ = io.WriteString(w, `{"type":"message","content":[{"type":"text","text":"{}"}]}`)
}

func post(t *testing.T, c *SamplingParamsFilter, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url+"/v1/messages", strings.NewReader(`{"model":"m","max_tokens":10,"temperature":0,"messages":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestModelRejectingTemperatureIsRetriedWithoutIt(t *testing.T) {
	fake := &fakeAnthropic{rejectTemperature: true}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	c := NewSamplingParamsFilter(srv.Client())

	if resp := post(t, c, srv.URL); resp.StatusCode != http.StatusOK {
		t.Fatalf("first request: status %d, want 200 after retry", resp.StatusCode)
	}
	if len(fake.bodies) != 2 {
		t.Fatalf("first request: %d calls, want 2 (rejected, then retried)", len(fake.bodies))
	}
	if _, ok := fake.bodies[1]["temperature"]; ok {
		t.Fatal("retry still carried temperature")
	}
	if fake.bodies[1]["max_tokens"] != float64(10) || fake.bodies[1]["model"] != "m" {
		t.Fatalf("retry lost other fields: %v", fake.bodies[1])
	}

	// Once learned, later requests go out without temperature straight away.
	if resp := post(t, c, srv.URL); resp.StatusCode != http.StatusOK {
		t.Fatalf("second request: status %d", resp.StatusCode)
	}
	if len(fake.bodies) != 3 {
		t.Fatalf("second request: %d calls in total, want 3 (no extra rejected round trip)", len(fake.bodies))
	}
	if _, ok := fake.bodies[2]["temperature"]; ok {
		t.Fatal("second request carried temperature")
	}
}

func TestModelAcceptingTemperatureKeepsIt(t *testing.T) {
	fake := &fakeAnthropic{}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	c := NewSamplingParamsFilter(srv.Client())

	post(t, c, srv.URL)
	post(t, c, srv.URL)
	if len(fake.bodies) != 2 {
		t.Fatalf("%d calls, want 2", len(fake.bodies))
	}
	for i, b := range fake.bodies {
		if _, ok := b["temperature"]; !ok {
			t.Fatalf("request %d lost temperature for a model that accepts it", i)
		}
	}
}

func TestOtherBadRequestsAreNotRetried(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"type":"error","error":{"type":"invalid_request_error","message":"prompt is too long"}}`)
	}))
	defer srv.Close()
	c := NewSamplingParamsFilter(srv.Client())

	resp := post(t, c, srv.URL)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d, want the original 400", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "prompt is too long") {
		t.Fatalf("original error body was not passed through: %s", body)
	}
	if calls != 1 {
		t.Fatalf("%d calls, want 1", calls)
	}
}
