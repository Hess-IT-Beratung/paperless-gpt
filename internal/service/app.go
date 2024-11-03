package service

import (
	"container/list"
	"context"
	_ "embed"
	"fmt"
	"os"
	"paperless-gpt/internal/logging"
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

	minBackoffDuration := 10 * time.Second
	maxBackoffDuration := time.Hour
	pollingInterval := 10 * time.Second

	backoffDuration := minBackoffDuration
	for {
		processedCount, err := app.processAutoTagDocuments()
		if err != nil {
			log.Errorf("Error in processAutoTagDocuments: %v", err)
			time.Sleep(backoffDuration)
			backoffDuration *= 2 // Exponential backoff
			if backoffDuration > maxBackoffDuration {
				log.Warnf("Repeated errors in processAutoTagDocuments detected. Setting backoff to %v", maxBackoffDuration)
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

// processAutoTagDocuments handles the background auto-tagging of documents
func (app *App) processAutoTagDocuments() (int, error) {
	ctx := context.Background()

	documents, err := app.PaperlessClient.GetDocumentsByTags(ctx, []string{config.AutoTag}, 1)
	if err != nil {
		return 0, fmt.Errorf("error fetching documents with autoTag: %w", err)
	}

	if len(documents) == 0 {
		log.Debugf("No documents with tag %s found", config.AutoTag)
		return 0, nil // No documents to process
	}

	log.Debugf("Found at least %d remaining documents with tag %s", len(documents), config.AutoTag)

	// Generate suggestion for the document
	suggestion, err := app.generateDocumentSuggestion(ctx, documents[0])
	if err != nil {
		return 0, fmt.Errorf("error generating suggestion: %w", err)
	}

	// Remove forbidden tags
	for _, tag := range config.ForbiddenTags {
		suggestion.Tags = paperless_service.RemoveTagFromList(suggestion.Tags, tag)
	}

	// Update document with suggestion
	err = app.PaperlessClient.UpdateDocument(ctx, *suggestion)
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
