package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"paperless-gpt/internal/logging"
	"paperless-gpt/paperless/paperless_model"
	"strings"
	"sync"
	"time"

	"paperless-gpt/internal/config"
)

var log = logging.InitLogger(config.LogLevel)

const (
	httpTimeout        = 15 * time.Second
	maxAttempts        = 3
	initialBackoff     = 1 * time.Second
	circuitThreshold   = 5
	circuitCooldown    = 60 * time.Second
	schemaVersion      = 1
)

type Payload struct {
	Schema        int       `json:"schema"`
	Event         string    `json:"event"`
	DocumentID    int       `json:"document_id"`
	Title         string    `json:"title,omitempty"`
	Tags          []string  `json:"tags,omitempty"`
	Correspondent     string    `json:"correspondent,omitempty"`
	DocumentType      string    `json:"document_type,omitempty"`
	Summary           string    `json:"summary,omitempty"`
	Urgency           string    `json:"urgency,omitempty"`            // low | medium | high | critical
	ActionRequired    bool      `json:"action_required"`
	ActionDescription string    `json:"action_description,omitempty"`
	Content           string    `json:"content,omitempty"`
	Timestamp         time.Time `json:"timestamp"`
}

// Enrichment is the LLM-generated metadata the caller hands to Notify.
type Enrichment struct {
	Summary           string
	Urgency           string
	ActionRequired    bool
	ActionDescription string
}

var validUrgencies = map[string]bool{
	"low":      true,
	"medium":   true,
	"high":     true,
	"critical": true,
}

type Notifier struct {
	baseURL string
	client  *http.Client

	mu               sync.Mutex
	consecutiveFails int
	openUntil        time.Time
}

func NewNotifier() *Notifier {
	return &Notifier{
		baseURL: strings.TrimRight(config.N8nWebhookURL, "/"),
		client:  &http.Client{Timeout: httpTimeout},
	}
}

func (n *Notifier) Enabled() bool { return n.baseURL != "" }

// Notify sends the suggestion to the configured webhook in a background goroutine.
// Fire-and-forget: errors are logged, never returned.
func (n *Notifier) Notify(ctx context.Context, event string, s *paperless_model.DocumentSuggestion, e Enrichment) {
	if !n.Enabled() || s == nil {
		return
	}
	urgency := e.Urgency
	if !validUrgencies[urgency] {
		urgency = "low"
	}
	actionDesc := e.ActionDescription
	if !e.ActionRequired {
		actionDesc = ""
	}
	p := Payload{
		Schema:            schemaVersion,
		Event:             event,
		DocumentID:        s.DocumentID,
		Summary:           e.Summary,
		Urgency:           urgency,
		ActionRequired:    e.ActionRequired,
		ActionDescription: actionDesc,
		Timestamp:         time.Now().UTC(),
	}
	if s.Title != nil {
		p.Title = *s.Title
	}
	if s.Tags != nil {
		p.Tags = *s.Tags
	}
	if s.Correspondent != nil {
		p.Correspondent = *s.Correspondent
	}
	if s.DocumentType != nil {
		p.DocumentType = *s.DocumentType
	}
	if s.Content != nil {
		p.Content = truncate(*s.Content, config.N8nWebhookContentMaxChars)
	}

	go n.send(ctx, event, p)
}

func (n *Notifier) send(ctx context.Context, event string, p Payload) {
	if !n.tryAcquire() {
		log.Debugf("webhook circuit open, skipping event=%s doc=%d", event, p.DocumentID)
		return
	}

	url := n.baseURL + "/" + event
	body, err := json.Marshal(p)
	if err != nil {
		log.Warnf("webhook payload marshal failed doc=%d: %v", p.DocumentID, err)
		return
	}

	backoff := initialBackoff
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		lastErr = n.attempt(ctx, url, body)
		if lastErr == nil {
			n.onSuccess()
			return
		}
		if isNonRetryable(lastErr) {
			break
		}
		if attempt == maxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			lastErr = ctx.Err()
			n.onFailure()
			log.Warnf("webhook delivery canceled event=%s doc=%d: %v", event, p.DocumentID, lastErr)
			return
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	n.onFailure()
	log.Warnf("webhook delivery gave up event=%s doc=%d url=%s err=%v", event, p.DocumentID, url, lastErr)
}

func (n *Notifier) attempt(ctx context.Context, url string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nonRetryable{err}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return nonRetryable{fmt.Errorf("http %d", resp.StatusCode)}
	}
	return fmt.Errorf("http %d", resp.StatusCode)
}

// truncate returns s cut to at most max runes, with an ellipsis marker when cut.
// A non-positive max disables truncation.
func truncate(s string, max int) string {
	if max <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…[truncated]"
}

type nonRetryable struct{ error }

func isNonRetryable(err error) bool {
	_, ok := err.(nonRetryable)
	return ok
}

// tryAcquire returns false if the circuit is currently open.
func (n *Notifier) tryAcquire() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	if !n.openUntil.IsZero() && time.Now().Before(n.openUntil) {
		return false
	}
	return true
}

func (n *Notifier) onSuccess() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.consecutiveFails > 0 || !n.openUntil.IsZero() {
		log.Debugf("webhook circuit closed after success")
	}
	n.consecutiveFails = 0
	n.openUntil = time.Time{}
}

func (n *Notifier) onFailure() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.consecutiveFails++
	if n.consecutiveFails >= circuitThreshold && n.openUntil.IsZero() {
		n.openUntil = time.Now().Add(circuitCooldown)
		log.Warnf("webhook circuit opened for %s after %d consecutive failures", circuitCooldown, n.consecutiveFails)
	}
}
