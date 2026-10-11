// Package llmhttp holds HTTP-level adjustments for the LLM provider clients.
package llmhttp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync/atomic"
)

// Doer is the HTTP client interface the langchaingo provider clients accept.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// SamplingParamsFilter removes the temperature from Anthropic requests once the API
// has rejected it.
//
// langchaingo always sends a temperature (0 when the caller sets none), but newer
// Claude models answer any request that carries one with a 400. Models that accept it
// keep receiving it unchanged, so their output stays as before. The first rejection
// costs one extra round trip; from then on the field is stripped from every request,
// which assumes one filter per configured model. Safe for concurrent use.
type SamplingParamsFilter struct {
	next Doer
	drop atomic.Bool
}

// NewSamplingParamsFilter wraps next, typically http.DefaultClient.
func NewSamplingParamsFilter(next Doer) *SamplingParamsFilter {
	return &SamplingParamsFilter{next: next}
}

func (f *SamplingParamsFilter) Do(req *http.Request) (*http.Response, error) {
	if req.Body == nil {
		return f.next.Do(req)
	}
	body, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, err
	}

	alreadyDropping := f.drop.Load()
	if alreadyDropping {
		body = withoutTemperature(body)
	}
	resp, err := f.next.Do(withBody(req, body))
	if err != nil || resp.StatusCode != http.StatusBadRequest || alreadyDropping {
		return resp, err
	}

	errBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if !bytes.Contains(errBody, []byte("temperature")) {
		resp.Body = io.NopCloser(bytes.NewReader(errBody))
		return resp, nil
	}
	f.drop.Store(true)
	return f.next.Do(withBody(req, withoutTemperature(body)))
}

// withoutTemperature returns body unchanged if it is not a JSON object.
func withoutTemperature(body []byte) []byte {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return body
	}
	if _, ok := fields["temperature"]; !ok {
		return body
	}
	delete(fields, "temperature")
	out, err := json.Marshal(fields)
	if err != nil {
		return body
	}
	return out
}

func withBody(req *http.Request, body []byte) *http.Request {
	r := req.Clone(req.Context())
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	return r
}
