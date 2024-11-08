package service

import (
	"container/list"
	"context"
	_ "embed"
	"fmt"
	"os"
	"paperless-gpt/internal/logging"
	"paperless-gpt/paperless/paperless_model"
	"paperless-gpt/paperless/paperless_service"
	"strings"
	"sync"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"

	"paperless-gpt/internal/config"
)

var (

	// Logger
	log = logging.InitLogger(config.LogLevel)
)

const maxCacheSize = 100 // Define the maximum size of the cache

// CacheEntry represents a single cache entry
type CacheEntry struct {
	key   string
	value string
}

// App struct to hold dependencies and cache
type App struct {
	PaperlessClient *paperless_service.PaperlessClient
	LlmClient       llms.Model
	cache           map[string]*list.Element
	cacheList       *list.List
	cacheMutex      sync.Mutex
}

func Start() {

	// Initialize PaperlessClient
	client := paperless_service.NewPaperlessClient(config.PaperlessBaseURL, config.PaperlessAPIToken)

	// Initialize LlmClient
	llm, err := createLLM()
	if err != nil {
		log.Fatalf("Failed to create LlmClient client: %v", err)
	}

	// Initialize App with dependencies
	app := &App{
		PaperlessClient: client,
		LlmClient:       llm,
		cache:           make(map[string]*list.Element),
		cacheList:       list.New(),
		cacheMutex:      sync.Mutex{},
	}

	var wg sync.WaitGroup
	errorChan := make(chan error, 2) // Buffered channel to capture errors

	wg.Add(2)

	go func() {
		defer wg.Done()
		if err := handleAutoTags(app, app.generateAutoDocumentSuggestion, config.AutoTag, "auto_tagged", true); err != nil {
			errorChan <- err
		}
	}()

	go func() {
		defer wg.Done()
		if err := handleAutoTags(app, app.getOcrDocumentSuggestion, config.OcrTag, "ocr_textract", false); err != nil {
			errorChan <- err
		}
	}()

	wg.Wait()
	close(errorChan) // Close the channel after all goroutines have completed

	// Handle errors
	for err := range errorChan {
		if err != nil {
			log.Errorf("Error occurred: %v", err)
		}
	}

}

func handleAutoTags(app *App, suggestionFunc SuggestionFunc, tagName string, customFieldName string, waitForOtherJobsToComplete bool) error {
	minBackoffDuration := 10 * time.Second
	maxBackoffDuration := time.Hour
	pollingInterval := 10 * time.Second

	backoffDuration := minBackoffDuration
	for {
		processedCount, err := app.processAutoTagDocuments(suggestionFunc, tagName, customFieldName, waitForOtherJobsToComplete)
		if err != nil {
			log.Errorf("Error in handleAutoTags: %v", err)
			time.Sleep(backoffDuration)
			backoffDuration *= 2 // Exponential backoff
			if backoffDuration > maxBackoffDuration {
				log.Warnf("Repeated errors in handleAutoTags detected. Setting backoff to %v", maxBackoffDuration)
				backoffDuration = maxBackoffDuration
			}
		} else {
			backoffDuration = minBackoffDuration
		}

		if processedCount == 0 {
			time.Sleep(pollingInterval)
		}
	}
}

// SuggestionFunc defines a function type that takes a document and returns a suggestion or error
type SuggestionFunc func(ctx context.Context, document paperless_model.Document) (*paperless_model.DocumentSuggestion, error)

// handles the background auto-tagging of documents
func (app *App) processAutoTagDocuments(suggestionFunc SuggestionFunc, tagName string, customFieldName string, tagLimit bool) (int, error) {
	ctx := context.Background()

	documents, err := app.PaperlessClient.GetDocumentsByTags(ctx, []string{tagName}, 1)
	if err != nil {
		return 0, fmt.Errorf("error fetching documents with autoTag: %w", err)
	}

	if len(documents) == 0 {
		log.Debugf("No documents with tag %s found", tagName)
		return 0, nil // No documents to process
	}

	log.Debugf("Found at least %d remaining documents with tag %s", len(documents), tagName)

	// Generate suggestion for the document using the provided function pointer
	document := documents[0]

	if tagLimit && len(document.Tags) >= 2 {
		log.Debugf("Document with id: '%d' still has more than one tag. Skipping auto-tagging.", document.ID)
		return 0, nil
	}

	suggestion, err := suggestionFunc(ctx, document)
	if err != nil {
		return 0, fmt.Errorf("error generating suggestion: %w", err)
	}

	*suggestion.Tags = paperless_service.RemoveTagFromList(*suggestion.Tags, tagName)

	// Update document with suggestion
	err = app.PaperlessClient.UpdateDocument(ctx, *suggestion, customFieldName)
	if err != nil {
		return 0, fmt.Errorf("error updating documents: %w", err)
	}

	return 1, nil
}

// createLLM creates the appropriate LlmClient client based on the provider
func createLLM() (llms.Model, error) {
	switch strings.ToLower(config.LlmProvider) {
	case "openai":
		if config.OpenaiAPIKey == "" {
			return nil, fmt.Errorf("OpenAI API key is not set")
		}
		return openai.New(
			openai.WithModel(config.LlmModel),
			openai.WithToken(config.OpenaiAPIKey),
		)
	case "ollama":
		host := os.Getenv("OLLAMA_HOST")
		if host == "" {
			host = "http://127.0.0.1:11434"
		}
		return ollama.New(
			ollama.WithModel(config.LlmModel),
			ollama.WithServerURL(host),
		)
	default:
		return nil, fmt.Errorf("unsupported LlmClient provider: %s", config.LlmProvider)
	}
}
