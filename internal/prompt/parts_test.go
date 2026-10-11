package prompt

import (
	"testing"

	"github.com/tmc/langchaingo/llms"
)

func TestCacheablePartsSplitsBeforeTheDocument(t *testing.T) {
	prompt := "rules and lists\n\n# Dokumentinhalt\n\nthe document\n\npostamble"
	parts := CacheableParts(prompt, "the document")
	if len(parts) != 2 {
		t.Fatalf("got %d parts, want 2", len(parts))
	}
	cached, ok := parts[0].(llms.CachedContent)
	if !ok || cached.CacheControl == nil || cached.CacheControl.Type != "ephemeral" {
		t.Fatalf("first part is not an ephemeral cached part: %#v", parts[0])
	}
	if cached.ContentPart.(llms.TextContent).Text != "rules and lists\n\n# Dokumentinhalt\n\n" {
		t.Fatalf("cached prefix = %q", cached.ContentPart.(llms.TextContent).Text)
	}
	if parts[1].(llms.TextContent).Text != "the document\n\npostamble" {
		t.Fatalf("rest = %q", parts[1].(llms.TextContent).Text)
	}
}

func TestCacheablePartsUsesTheLastOccurrence(t *testing.T) {
	// A document whose text also appears earlier (e.g. inside a list) must still be split before the document itself.
	parts := CacheableParts("Liste: Rechnung\n\nRechnung", "Rechnung")
	if got := parts[0].(llms.CachedContent).ContentPart.(llms.TextContent).Text; got != "Liste: Rechnung\n\n" {
		t.Fatalf("cached prefix = %q", got)
	}
}

func TestCacheablePartsFallsBackToOnePart(t *testing.T) {
	for _, tc := range []struct{ prompt, content string }{
		{"template without the content", "the document"}, // custom template without {{.Content}}
		{"the document", "the document"},                 // nothing before the document worth caching
		{"prefix", ""},
	} {
		parts := CacheableParts(tc.prompt, tc.content)
		if len(parts) != 1 || parts[0].(llms.TextContent).Text != tc.prompt {
			t.Fatalf("CacheableParts(%q, %q) = %#v, want the prompt as one part", tc.prompt, tc.content, parts)
		}
	}
}
