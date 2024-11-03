package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"paperless-gpt/internal/config"
	"paperless-gpt/paperless/paperless_model"
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

	log.Debugf("Json suggestion: %s", jsonStr)

	var suggestion paperless_model.DocumentSuggestion
	err = json.Unmarshal([]byte(jsonStr), &suggestion)
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

	// Generate json suggestion
	if jsonSuggestion, err := app.getSuggestedJson(ctx, content, availableTagNames, availableCorrespondentNames, config.CorrespondentBlackList, availableDocumentTypeNames, doc); err != nil {
		return nil, fmt.Errorf("error generating json for document %d: %v", documentID, err)
	} else {
		return jsonSuggestion, nil
	}

}
