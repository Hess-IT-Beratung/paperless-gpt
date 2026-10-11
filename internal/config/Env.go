package config

import (
	"os"
	"paperless-gpt/internal/logging"
	"strconv"
	"strings"
)

var (
	log = logging.InitLogger(LogLevel)

	PaperlessBaseURL  = os.Getenv("PAPERLESS_BASE_URL")
	PaperlessAPIToken = os.Getenv("PAPERLESS_API_TOKEN")
	OpenaiAPIKey      = os.Getenv("OPENAI_API_KEY")
	AnthropicAPIKey   = os.Getenv("ANTHROPIC_API_KEY")
	AutoTag           = os.Getenv("PAPERLESS_AUTO_TAG")
	OcrTag            = os.Getenv("PAPERLESS_OCR_TAG")
	LlmProvider       = os.Getenv("LLM_PROVIDER")
	LlmModel          = os.Getenv("LLM_MODEL")
	// LlmMaxTokens caps the answer of Anthropic models, thinking included. Thinking models
	// can spend more than langchaingo's default of 2048 on thinking alone and then return
	// a cut-off answer. Only output actually generated is billed, so a high cap is cheap.
	LlmMaxTokens           = intEnvVar("LLM_MAX_TOKENS", 16000)
	LogLevel               = strings.ToLower(os.Getenv("LOG_LEVEL"))
	CorrespondentBlackList = splitEnvVar("CORRESPONDENT_BLACK_LIST")
	TagBlackList           = splitEnvVar("TAG_BLACK_LIST")

	Region = os.Getenv("AWS_REGION")
	Bucket = os.Getenv("AWS_OCR_BUCKET_NAME")

	N8nWebhookURL             = os.Getenv("N8N_WEBHOOK_URL")
	N8nWebhookContentMaxChars = intEnvVar("N8N_WEBHOOK_CONTENT_MAX_CHARS", 5000)

	PromptPreamble           = os.Getenv("PROMPT_PREAMBLE")
	TitleExplanation         = os.Getenv("PROMPT_TITLE_EXPLANATION")
	TagsExplanation          = os.Getenv("PROMPT_TAGS_EXPLANATION")
	DocumentTypeExplanation  = os.Getenv("PROMPT_DOCUMENT_TYPE_EXPLANATION")
	CorrespondentExplanation = os.Getenv("PROMPT_CORRESPONDENT_EXPLANATION")
	PromptPostamble          = os.Getenv("PROMPT_POSTAMBLE")
)

func init() {
	validateEnvVars()
}

// intEnvVar reads an env var as int, returning the fallback when unset or invalid.
func intEnvVar(name string, fallback int) int {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// splitEnvVar splits an environment variable by commas and returns a slice of strings
func splitEnvVar(envVar string) []string {
	value := os.Getenv(envVar)
	if value == "" {
		return []string{}
	}
	return strings.Split(value, ",")
}

// validateEnvVars ensures all necessary environment variables are set
func validateEnvVars() {
	if PaperlessBaseURL == "" {
		log.Fatal("Please set the PAPERLESS_BASE_URL environment variable.")
	}

	if PaperlessAPIToken == "" {
		log.Fatal("Please set the PAPERLESS_API_TOKEN environment variable.")
	}

	if LlmProvider == "" {
		log.Fatal("Please set the LLM_PROVIDER environment variable.")
	}

	if LlmModel == "" {
		log.Fatal("Please set the LLM_MODEL environment variable.")
	}

	if LlmProvider == "openai" && OpenaiAPIKey == "" {
		log.Fatal("Please set the OPENAI_API_KEY environment variable for OpenAI provider.")
	}

	if LlmProvider == "anthropic" && AnthropicAPIKey == "" {
		log.Fatal("Please set the ANTHROPIC_API_KEY environment variable for Anthropic provider.")
	}

	if Region == "" {
		log.Fatal("missing environment variable: AWS_REGION")
	}
	if Bucket == "" {
		log.Fatal("missing environment variable: AWS_OCR_BUCKET_NAME")
	}

	if AutoTag == "" {
		AutoTag = "paperless-gpt-auto"
	}
	if OcrTag == "" {
		OcrTag = "paperless-gpt-ocr"
	}

	if len(TagBlackList) == 0 {
		TagBlackList = append(TagBlackList, OcrTag)
	}
}

// getLikelyLanguage determines the likely language of the document content
func GetLikelyLanguage() string {
	likelyLanguage := os.Getenv("LLM_LANGUAGE")
	if likelyLanguage == "" {
		likelyLanguage = "English"
	}
	return strings.Title(strings.ToLower(likelyLanguage))
}
