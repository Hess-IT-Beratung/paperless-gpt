package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"paperless-gpt/internal/config"
	"paperless-gpt/paperless/paperless_model"
	"sort"
	"strings"

	"github.com/tmc/langchaingo/llms"
)

// getSuggestedJson generates a suggested json for a document using the LlmClient
func (app *App) getSuggestedJson(ctx context.Context, content string, availableTags []string, availableCorrespondents []string, correspondentBlackList []string, availableDocumentTypeNames []string, originalDocument paperless_model.Document) (*paperless_model.DocumentSuggestion, error) {
	likelyLanguage := config.GetLikelyLanguage()

	var promptBuffer bytes.Buffer
	err := config.JsonPrompt.Execute(&promptBuffer, map[string]interface{}{
		"Language":                likelyLanguage,
		"AvailableTags":           availableTags,
		"AvailableCorrespondents": availableCorrespondents,
		"BlackList":               correspondentBlackList,
		"Content":                 content,
		"AvailableDocumentTypes":  availableDocumentTypeNames,
	})
	if err != nil {
		return nil, fmt.Errorf("error executing json template: %v", err)
	}

	prompt := promptBuffer.String()
	log.Debugf("Json suggestion prompt: %s", prompt)

	// Check cache
	app.cacheMutex.Lock()
	if element, found := app.cache[prompt]; found {
		app.cacheList.MoveToFront(element)
		app.cacheMutex.Unlock()
		log.Warnf("Cache hit for prompt of document %d", originalDocument.ID)
		return unmarshalSuggestion(element.Value.(*CacheEntry).value, originalDocument)
	} else {
		log.Debugf("Cache miss for prompt of document %d", originalDocument.ID)
	}
	app.cacheMutex.Unlock()

	// Generate content
	completion, err := app.LlmClient.GenerateContent(ctx, []llms.MessageContent{
		{
			Parts: []llms.ContentPart{
				llms.TextContent{
					Text: prompt,
				},
			},
			Role: llms.ChatMessageTypeHuman,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error getting response from LlmClient: %v", err)
	}

	jsonStr := strings.TrimSpace(completion.Choices[0].Content)
	log.Infof("Json suggestion for document %d: %s", originalDocument.ID, jsonStr)

	// Store in cache
	app.cacheMutex.Lock()
	if app.cacheList.Len() >= maxCacheSize {
		// Evict the least recently used entry
		evictElement := app.cacheList.Back()
		if evictElement != nil {
			app.cacheList.Remove(evictElement)
			delete(app.cache, evictElement.Value.(*CacheEntry).key)
		}
	}
	newEntry := &CacheEntry{key: prompt, value: jsonStr}
	element := app.cacheList.PushFront(newEntry)
	app.cache[prompt] = element
	log.Debugf("Added prompt to cache of document %d", originalDocument.ID)
	app.cacheMutex.Unlock()

	return unmarshalSuggestion(jsonStr, originalDocument)
}

func unmarshalSuggestion(jsonStr string, originalDocument paperless_model.Document) (*paperless_model.DocumentSuggestion, error) {

	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "" {
		return nil, fmt.Errorf("error: json string is empty or blank")
	}

	// the json string might begin with invalid characters (Markdown). Remove them until the first '{'
	for i, c := range jsonStr {
		if c == '{' {
			jsonStr = jsonStr[i:]
			break
		}
	}

	// the json string might end with invalid characters. Remove them until the last '}'
	for i := len(jsonStr) - 1; i >= 0; i-- {
		if jsonStr[i] == '}' {
			jsonStr = jsonStr[:i+1]
			break
		}
	}

	var suggestion paperless_model.DocumentSuggestion
	err := json.Unmarshal([]byte(jsonStr), &suggestion)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling json: %v", err)
	}
	suggestion.DocumentID = originalDocument.ID
	suggestion.OriginalDocument = originalDocument
	return &suggestion, nil
}

// generateDocumentSuggestion generates suggestions (title, tags, and correspondent) for a single document.
func (app *App) generateDocumentSuggestion(ctx context.Context, doc paperless_model.Document) (*paperless_model.DocumentSuggestion, error) {
	// Fetch all available tags from paperless-ngx
	availableTagsMap, err := app.PaperlessClient.GetAllTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch available tags: %v", err)
	}

	availableTagNames := make([]string, 0, len(availableTagsMap))
	for tagName := range availableTagsMap {
		availableTagNames = append(availableTagNames, tagName)
	}

	// Fetch all available correspondents from paperless-ngx
	availableCorrespondentsMap, err := app.PaperlessClient.GetAllCorrespondents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch available correspondents: %v", err)
	}
	availableCorrespondentNames := make([]string, 0, len(availableCorrespondentsMap))
	for correspondentName := range availableCorrespondentsMap {
		availableCorrespondentNames = append(availableCorrespondentNames, correspondentName)
	}

	// Fetch all available document types from paperless-ngx
	availableDocumentTypesMap, err := app.PaperlessClient.GetAllDocumentTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch available document types: %v", err)
	}
	availableDocumentTypeNames := make([]string, 0, len(availableDocumentTypesMap))
	for documentTypeName := range availableDocumentTypesMap {
		availableDocumentTypeNames = append(availableDocumentTypeNames, documentTypeName)
	}

	// Prepare for generating suggestions
	documentID := doc.ID
	content := doc.Content
	if len(content) > 5000 {
		content = content[:5000]
	}

	// Sort the names for consistency (Important for caching)
	sort.Strings(availableTagNames)
	sort.Strings(availableCorrespondentNames)
	sort.Strings(availableDocumentTypeNames)

	// Generate json suggestion
	if jsonSuggestion, err := app.getSuggestedJson(ctx, content, availableTagNames, availableCorrespondentNames, config.CorrespondentBlackList, availableDocumentTypeNames, doc); err != nil {
		return nil, fmt.Errorf("error generating json for document %d: %v", documentID, err)
	} else {
		return jsonSuggestion, nil
	}

}

func sortStrings(names []string) []string {
	sort.Strings(names)
	return names
}
