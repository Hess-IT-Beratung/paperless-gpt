// Package prompt parses the prompt template and prepares the rendered prompt for the LLM.
package prompt

import (
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/tmc/langchaingo/llms"
)

// Parse parses a prompt template with the sprig functions.
//
// It is a text template on purpose: the prompt is plain text for a model, and HTML
// escaping would turn "&" into "&amp;" in every list name and document. The model is
// told to copy names character by character, so it would then create correspondents
// and tags with the escaped names.
func Parse(name, text string) (*template.Template, error) {
	return template.New(name).Funcs(sprig.TxtFuncMap()).Parse(text)
}

// CacheableParts splits a rendered prompt right before the document content, so that
// the part in front of it (instructions and the lists of correspondents, types and
// tags, identical for every document) can be cached by the provider.
//
// The last occurrence of content is used because the document comes last in the
// template; an earlier match would be a list entry that happens to equal the text.
// Without a usable split (content missing from the template, or nothing in front of
// it) the prompt is returned as a single uncached part.
func CacheableParts(rendered, content string) []llms.ContentPart {
	i := -1
	if content != "" {
		i = strings.LastIndex(rendered, content)
	}
	if i <= 0 || strings.TrimSpace(rendered[:i]) == "" {
		return []llms.ContentPart{llms.TextContent{Text: rendered}}
	}
	return []llms.ContentPart{
		llms.WithCacheControl(llms.TextContent{Text: rendered[:i]}, &llms.CacheControl{Type: "ephemeral"}),
		llms.TextContent{Text: rendered[i:]},
	}
}
