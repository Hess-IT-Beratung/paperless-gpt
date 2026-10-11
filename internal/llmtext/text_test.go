package llmtext

import (
	"testing"

	"github.com/tmc/langchaingo/llms"
)

func TestTextSkipsEmptyThinkingChoices(t *testing.T) {
	// Anthropic models that think return a thinking block first; langchaingo maps it to a
	// choice with empty Content, followed by the choice that holds the actual answer.
	resp := &llms.ContentResponse{Choices: []*llms.ContentChoice{
		{Content: "", GenerationInfo: map[string]any{"ThinkingContent": ""}},
		{Content: `{"title":"x"}`},
	}}
	if got := Text(resp); got != `{"title":"x"}` {
		t.Fatalf("Text() = %q", got)
	}
}

func TestTextJoinsSeveralTextChoices(t *testing.T) {
	resp := &llms.ContentResponse{Choices: []*llms.ContentChoice{{Content: "a"}, {Content: ""}, {Content: "b"}}}
	if got := Text(resp); got != "a\nb" {
		t.Fatalf("Text() = %q", got)
	}
}

func TestTextOfEmptyResponse(t *testing.T) {
	for _, resp := range []*llms.ContentResponse{nil, {}, {Choices: []*llms.ContentChoice{nil, {Content: "  "}}}} {
		if got := Text(resp); got != "" {
			t.Fatalf("Text(%v) = %q, want empty", resp, got)
		}
	}
}

func TestLastJSONObjectTakesTheCorrectedAnswer(t *testing.T) {
	// A model that corrects itself mid-answer emits an invalid first object, prose, then the valid one.
	s := "{\n  \"title\": \"first\",\n  \"tags_note\": null\n}\n\nKorrektur: Ausgabe nur im Schema. Hier die gültige Antwort:\n\n{\n  \"title\": \"second\"\n}"
	got, err := LastJSONObject(s)
	if err != nil || got != "{\n  \"title\": \"second\"\n}" {
		t.Fatalf("LastJSONObject() = %q, %v", got, err)
	}
}

func TestLastJSONObjectInMarkdownFence(t *testing.T) {
	got, err := LastJSONObject("```json\n{\"a\": {\"b\": \"}\"}}\n```")
	if err != nil || got != `{"a": {"b": "}"}}` {
		t.Fatalf("LastJSONObject() = %q, %v", got, err)
	}
}

func TestLastJSONObjectWithoutObject(t *testing.T) {
	for _, s := range []string{"", "no json here", "{broken"} {
		if got, err := LastJSONObject(s); err == nil {
			t.Fatalf("LastJSONObject(%q) = %q, want an error", s, got)
		}
	}
}
