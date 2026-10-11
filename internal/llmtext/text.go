// Package llmtext extracts the answer text and the JSON payload from LLM responses.
package llmtext

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/tmc/langchaingo/llms"
)

// Text returns the text of all non-empty choices, joined by newlines.
//
// Reading only the first choice is not enough: when an Anthropic model thinks,
// langchaingo maps the thinking block to a choice with empty Content that comes
// before the choice holding the actual answer.
func Text(resp *llms.ContentResponse) string {
	if resp == nil {
		return ""
	}
	var parts []string
	for _, c := range resp.Choices {
		if c != nil && strings.TrimSpace(c.Content) != "" {
			parts = append(parts, strings.TrimSpace(c.Content))
		}
	}
	return strings.Join(parts, "\n")
}

// LastJSONObject returns the last complete top-level JSON object in s.
//
// The last one wins because a model that corrects itself mid-answer emits a
// rejected object, some prose and then the object it means. Text around the
// objects, such as a Markdown code fence, is ignored.
func LastJSONObject(s string) (string, error) {
	var last json.RawMessage
	for i := 0; i < len(s); {
		start := strings.IndexByte(s[i:], '{')
		if start < 0 {
			break
		}
		start += i
		dec := json.NewDecoder(strings.NewReader(s[start:]))
		var obj json.RawMessage
		if err := dec.Decode(&obj); err != nil || !bytes.HasPrefix(obj, []byte("{")) {
			i = start + 1
			continue
		}
		last = obj
		i = start + int(dec.InputOffset())
	}
	if last == nil {
		return "", errors.New("no JSON object found")
	}
	return string(last), nil
}
