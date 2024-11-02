package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/sirupsen/logrus"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"
)

// Global Variables and Constants
var (

	// Logger
	log = logrus.New()

	// Environment Variables
	paperlessBaseURL       = os.Getenv("PAPERLESS_BASE_URL")
	paperlessAPIToken      = os.Getenv("PAPERLESS_API_TOKEN")
	openaiAPIKey           = os.Getenv("OPENAI_API_KEY")
	manualTag              = "paperless-gpt"
	autoTag                = "paperless-gpt-auto"
	llmProvider            = os.Getenv("LLM_PROVIDER")
	llmModel               = os.Getenv("LLM_MODEL")
	logLevel               = strings.ToLower(os.Getenv("LOG_LEVEL"))
	correspondentBlackList = strings.Split(os.Getenv("CORRESPONDENT_BLACK_LIST"), ",")

	// Templates
	titleTemplate         *template.Template
	tagTemplate           *template.Template
	correspondentTemplate *template.Template
	templateMutex         sync.RWMutex

	// Default templates
	defaultTitleTemplate = `I will provide you with the content of a document that has been partially read by OCR (so it may contain errors).
Your task is to find a suitable document title that I can use as the title in the paperless-ngx program.
Respond only with the title, without any additional information. The content is likely in {{.Language}}.

Content:
{{.Content}}
`

	defaultTagTemplate = `I will provide you with the content and the title of a document. Your task is to select appropriate tags for the document from the list of available tags I will provide. Only select tags from the provided list. Respond only with the selected tags as a comma-separated list, without any additional information. The content is likely in {{.Language}}.

Available Tags:
{{.AvailableTags | join ", "}}

Title:
{{.Title}}

Content:
{{.Content}}

Please concisely select the {{.Language}} tags from the list above that best describe the document.
Be very selective and only choose the most relevant tags since too many tags will make the document less discoverable.
`

	defaultCorrespondentTemplate = `I will provide you with the content of a document. Your task is to suggest a correspondent that is most relevant to the document.

Correspondents are the senders of documents that reach you. In the other direction, correspondents are the recipients of documents that you send.
In Paperless-ngx we can imagine correspondents as virtual drawers in which all documents of a person or company are stored. With just one click, we can find all the documents assigned to a specific correspondent.
Try to suggest a correspondent, either from the example list or come up with a new correspondent.

Respond only with a correspondent, without any additional information!

Be sure to choose a correspondent that is most relevant to the document.
Try to avoid any legal or financial suffixes like "GmbH" or "AG" in the correspondent name. For example use "Microsoft" instead of "Microsoft Ireland Operations Limited" or "Amazon" instead of "Amazon EU S.a.r.l.".

If you can't find a suitable correspondent, you can respond with "Unknown".

Example Correspondents:
{{.AvailableCorrespondents | join ", "}}

List of Correspondents with Blacklisted Names. Please avoid these correspondents or variations of their names:
{{.BlackList | join ", "}}

Title of the document:
{{.Title}}

The content is likely in {{.Language}}.

Document Content:
{{.Content}}
`
)

// App struct to hold dependencies
type App struct {
	Client *PaperlessClient
	LLM    llms.Model
}

func main() {
	// Validate Environment Variables
	validateEnvVars()

	// Initialize logrus logger
	initLogger()

	// Initialize PaperlessClient
	client := NewPaperlessClient(paperlessBaseURL, paperlessAPIToken)

	// Load Templates
	loadTemplates()

	// Initialize LLM
	llm, err := createLLM()
	if err != nil {
		log.Fatalf("Failed to create LLM client: %v", err)
	}

	// Initialize App with dependencies
	app := &App{
		Client: client,
		LLM:    llm,
	}

	// Start background process for auto-tagging
	go func() {
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
	}()

}

func initLogger() {
	switch logLevel {
	case "debug":
		log.SetLevel(logrus.DebugLevel)
	case "info":
		log.SetLevel(logrus.InfoLevel)
	case "warn":
		log.SetLevel(logrus.WarnLevel)
	case "error":
		log.SetLevel(logrus.ErrorLevel)
	default:
		log.SetLevel(logrus.InfoLevel)
		if logLevel != "" {
			log.Fatalf("Invalid log level: '%s'.", logLevel)
		}
	}

	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

// validateEnvVars ensures all necessary environment variables are set
func validateEnvVars() {
	if paperlessBaseURL == "" {
		log.Fatal("Please set the PAPERLESS_BASE_URL environment variable.")
	}

	if paperlessAPIToken == "" {
		log.Fatal("Please set the PAPERLESS_API_TOKEN environment variable.")
	}

	if llmProvider == "" {
		log.Fatal("Please set the LLM_PROVIDER environment variable.")
	}

	if llmModel == "" {
		log.Fatal("Please set the LLM_MODEL environment variable.")
	}

	if llmProvider == "openai" && openaiAPIKey == "" {
		log.Fatal("Please set the OPENAI_API_KEY environment variable for OpenAI provider.")
	}
}

// processAutoTagDocuments handles the background auto-tagging of documents
func (app *App) processAutoTagDocuments() (int, error) {
	ctx := context.Background()

	documents, err := app.Client.GetDocumentsByTags(ctx, []string{autoTag}, 1)
	if err != nil {
		return 0, fmt.Errorf("error fetching documents with autoTag: %w", err)
	}

	if len(documents) == 0 {
		log.Debugf("No documents with tag %s found", autoTag)
		return 0, nil // No documents to process
	}

	log.Debugf("Found at least %d remaining documents with tag %s", len(documents), autoTag)

	suggestionRequest := GenerateSuggestionsRequest{
		Documents:              documents,
		GenerateTitles:         true,
		GenerateTags:           true,
		GenerateCorrespondents: true,
	}

	suggestions, err := app.generateDocumentSuggestions(ctx, suggestionRequest)
	if err != nil {
		return 0, fmt.Errorf("error generating suggestions: %w", err)
	}

	err = app.Client.UpdateDocuments(ctx, suggestions)
	if err != nil {
		return 0, fmt.Errorf("error updating documents: %w", err)
	}

	return len(documents), nil
}

// removeTagFromList removes a specific tag from a list of tags
func removeTagFromList(tags []string, tagToRemove string) []string {
	filteredTags := []string{}
	for _, tag := range tags {
		if tag != tagToRemove {
			filteredTags = append(filteredTags, tag)
		}
	}
	return filteredTags
}

// getLikelyLanguage determines the likely language of the document content
func getLikelyLanguage() string {
	likelyLanguage := os.Getenv("LLM_LANGUAGE")
	if likelyLanguage == "" {
		likelyLanguage = "English"
	}
	return strings.Title(strings.ToLower(likelyLanguage))
}

// loadTemplates loads the title and tag templates from files or uses default templates
func loadTemplates() {
	templateMutex.Lock()
	defer templateMutex.Unlock()

	// Ensure prompts directory exists
	promptsDir := "prompts"
	if err := os.MkdirAll(promptsDir, os.ModePerm); err != nil {
		log.Fatalf("Failed to create prompts directory: %v", err)
	}

	// Load title template
	titleTemplatePath := filepath.Join(promptsDir, "title_prompt.tmpl")
	titleTemplateContent, err := os.ReadFile(titleTemplatePath)
	if err != nil {
		log.Errorf("Could not read %s, using default template: %v", titleTemplatePath, err)
		titleTemplateContent = []byte(defaultTitleTemplate)
		if err := os.WriteFile(titleTemplatePath, titleTemplateContent, os.ModePerm); err != nil {
			log.Fatalf("Failed to write default title template to disk: %v", err)
		}
	}
	titleTemplate, err = template.New("title").Funcs(sprig.FuncMap()).Parse(string(titleTemplateContent))
	if err != nil {
		log.Fatalf("Failed to parse title template: %v", err)
	}

	// Load tag template
	tagTemplatePath := filepath.Join(promptsDir, "tag_prompt.tmpl")
	tagTemplateContent, err := os.ReadFile(tagTemplatePath)
	if err != nil {
		log.Errorf("Could not read %s, using default template: %v", tagTemplatePath, err)
		tagTemplateContent = []byte(defaultTagTemplate)
		if err := os.WriteFile(tagTemplatePath, tagTemplateContent, os.ModePerm); err != nil {
			log.Fatalf("Failed to write default tag template to disk: %v", err)
		}
	}
	tagTemplate, err = template.New("tag").Funcs(sprig.FuncMap()).Parse(string(tagTemplateContent))
	if err != nil {
		log.Fatalf("Failed to parse tag template: %v", err)
	}

	// Load correspondent template
	correspondentTemplatePath := filepath.Join(promptsDir, "correspondent_prompt.tmpl")
	correspondentTemplateContent, err := os.ReadFile(correspondentTemplatePath)
	if err != nil {
		log.Errorf("Could not read %s, using default template: %v", correspondentTemplatePath, err)
		correspondentTemplateContent = []byte(defaultCorrespondentTemplate)
		if err := os.WriteFile(correspondentTemplatePath, correspondentTemplateContent, os.ModePerm); err != nil {
			log.Fatalf("Failed to write default correspondent template to disk: %v", err)
		}
	}
	correspondentTemplate, err = template.New("correspondent").Funcs(sprig.FuncMap()).Parse(string(correspondentTemplateContent))
	if err != nil {
		log.Fatalf("Failed to parse correspondent template: %v", err)
	}

}

// createLLM creates the appropriate LLM client based on the provider
func createLLM() (llms.Model, error) {
	switch strings.ToLower(llmProvider) {
	case "openai":
		if openaiAPIKey == "" {
			return nil, fmt.Errorf("OpenAI API key is not set")
		}
		return openai.New(
			openai.WithModel(llmModel),
			openai.WithToken(openaiAPIKey),
		)
	case "ollama":
		host := os.Getenv("OLLAMA_HOST")
		if host == "" {
			host = "http://127.0.0.1:11434"
		}
		return ollama.New(
			ollama.WithModel(llmModel),
			ollama.WithServerURL(host),
		)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", llmProvider)
	}
}
