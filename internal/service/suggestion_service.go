package service

import (
	"bytes"
	"context"
	"fmt"
	"paperless-gpt/internal/config"
	"paperless-gpt/paperless/paperless_model"
	"strings"
	"sync"

	"paperless-gpt/internal/model"

	"github.com/tmc/langchaingo/llms"
)

// getSuggestedCorrespondent generates a suggested correspondent for a document using the LlmClient
func (app *App) getSuggestedCorrespondent(ctx context.Context, content string, suggestedTitle string, availableCorrespondents []string, correspondentBlackList []string) (string, error) {
	likelyLanguage := config.GetLikelyLanguage()

	var promptBuffer bytes.Buffer
	err := config.CorrespondentPrompt.Execute(&promptBuffer, map[string]interface{}{
		"Language":                likelyLanguage,
		"AvailableCorrespondents": availableCorrespondents,
		"BlackList":               correspondentBlackList,
		"Title":                   suggestedTitle,
		"Content":                 content,
	})
	if err != nil {
		return "", fmt.Errorf("error executing correspondent template: %v", err)
	}

	prompt := promptBuffer.String()
	log.Debugf("Correspondent suggestion prompt: %s", prompt)

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
		return "", fmt.Errorf("error getting response from LlmClient: %v", err)
	}

	response := strings.TrimSpace(completion.Choices[0].Content)
	return response, nil
}

// getSuggestedTags generates suggested tags for a document using the LlmClient
func (app *App) getSuggestedTags(ctx context.Context, content string, suggestedTitle string, availableTags []string) ([]string, error) {
	likelyLanguage := config.GetLikelyLanguage()

	var promptBuffer bytes.Buffer
	err := config.TagPrompt.Execute(&promptBuffer, map[string]interface{}{
		"Language":      likelyLanguage,
		"AvailableTags": availableTags,
		"Title":         suggestedTitle,
		"Content":       content,
	})
	if err != nil {
		return nil, fmt.Errorf("error executing tag template: %v", err)
	}

	prompt := promptBuffer.String()
	log.Debugf("Tag suggestion prompt: %s", prompt)

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

	response := strings.TrimSpace(completion.Choices[0].Content)
	suggestedTags := strings.Split(response, ",")
	for i, tag := range suggestedTags {
		suggestedTags[i] = strings.TrimSpace(tag)
	}

	// Filter out tags that are not in the available tags list
	filteredTags := []string{}
	for _, tag := range suggestedTags {
		for _, availableTag := range availableTags {
			if strings.EqualFold(tag, availableTag) {
				filteredTags = append(filteredTags, availableTag)
				break
			}
		}
	}

	return filteredTags, nil
}

// getSuggestedTitle generates a suggested title for a document using the LlmClient
func (app *App) getSuggestedTitle(ctx context.Context, content string) (string, error) {
	likelyLanguage := config.GetLikelyLanguage()

	var promptBuffer bytes.Buffer
	err := config.TitlePrompt.Execute(&promptBuffer, map[string]interface{}{
		"Language": likelyLanguage,
		"Content":  content,
	})
	if err != nil {
		return "", fmt.Errorf("error executing title template: %v", err)
	}

	prompt := promptBuffer.String()

	log.Debugf("Title suggestion prompt: %s", prompt)

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
		return "", fmt.Errorf("error getting response from LlmClient: %v", err)
	}

	return strings.TrimSpace(strings.Trim(completion.Choices[0].Content, "\"")), nil
}

// generateDocumentSuggestions generates suggestions for a set of documents
func (app *App) generateDocumentSuggestions(ctx context.Context, suggestionRequest model.GenerateSuggestionsRequest) ([]paperless_model.DocumentSuggestion, error) {
	// Fetch all available tags from paperless-ngx
	availableTagsMap, err := app.PaperlessClient.GetAllTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch available tags: %v", err)
	}

	// Prepare a list of tag names
	availableTagNames := make([]string, 0, len(availableTagsMap))
	for tagName := range availableTagsMap {
		availableTagNames = append(availableTagNames, tagName)
	}

	// Prepare a list of document correspodents
	availableCorrespondentsMap, err := app.PaperlessClient.GetAllCorrespondents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch available correspondents: %v", err)
	}

	// Prepare a list of correspondent names
	availableCorrespondentNames := make([]string, 0, len(availableCorrespondentsMap))
	for correspondentName := range availableCorrespondentsMap {
		availableCorrespondentNames = append(availableCorrespondentNames, correspondentName)
	}

	documents := suggestionRequest.Documents
	documentSuggestions := []paperless_model.DocumentSuggestion{}

	var wg sync.WaitGroup
	var mu sync.Mutex
	errorsList := make([]error, 0)

	for i := range documents {
		wg.Add(1)
		go func(doc paperless_model.Document) {
			defer wg.Done()
			documentID := doc.ID
			log.Printf("Processing Document ID %d...", documentID)

			content := doc.Content
			if len(content) > 5000 {
				content = content[:5000]
			}

			var suggestedTitle string
			var suggestedTags []string
			var suggestedCorrespondent string

			if suggestionRequest.GenerateTitles {
				suggestedTitle, err = app.getSuggestedTitle(ctx, content)
				if err != nil {
					mu.Lock()
					errorsList = append(errorsList, fmt.Errorf("Document %d: %v", documentID, err))
					mu.Unlock()
					log.Errorf("Error processing document %d: %v", documentID, err)
					return
				}
			}

			if suggestionRequest.GenerateTags {
				suggestedTags, err = app.getSuggestedTags(ctx, content, suggestedTitle, availableTagNames)
				if err != nil {
					mu.Lock()
					errorsList = append(errorsList, fmt.Errorf("Document %d: %v", documentID, err))
					mu.Unlock()
					log.Errorf("Error generating tags for document %d: %v", documentID, err)
					return
				}
			}

			if suggestionRequest.GenerateCorrespondents {
				suggestedCorrespondent, err = app.getSuggestedCorrespondent(ctx, content, suggestedTitle, availableCorrespondentNames, config.CorrespondentBlackList)
				if err != nil {
					mu.Lock()
					errorsList = append(errorsList, fmt.Errorf("Document %d: %v", documentID, err))
					mu.Unlock()
					log.Errorf("Error generating correspondents for document %d: %v", documentID, err)
					return
				}

			}

			mu.Lock()
			suggestion := paperless_model.DocumentSuggestion{
				ID:               documentID,
				OriginalDocument: doc,
			}
			// Titles
			if suggestionRequest.GenerateTitles {
				log.Printf("Suggested title for document %d: %s", documentID, suggestedTitle)
				suggestion.SuggestedTitle = suggestedTitle
			} else {
				suggestion.SuggestedTitle = doc.Title
			}

			// Tags
			if suggestionRequest.GenerateTags {
				log.Printf("Suggested tags for document %d: %v", documentID, suggestedTags)
				suggestion.SuggestedTags = suggestedTags
			}

			// Correspondents
			if suggestionRequest.GenerateCorrespondents {
				log.Printf("Suggested correspondent for document %d: %s", documentID, suggestedCorrespondent)
				suggestion.SuggestedCorrespondent = suggestedCorrespondent
			} else {
				suggestion.SuggestedCorrespondent = ""
			}

			documentSuggestions = append(documentSuggestions, suggestion)
			mu.Unlock()
			log.Printf("Document %d processed successfully.", documentID)
		}(documents[i])
	}

	wg.Wait()

	if len(errorsList) > 0 {
		return nil, errorsList[0] // Return the first error encountered
	}

	return documentSuggestions, nil
}
