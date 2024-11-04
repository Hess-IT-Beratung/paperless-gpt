package config

import (
	"os"
	"paperless-gpt/internal/logging"
	"strings"
)

var (
	log = logging.InitLogger(LogLevel)

	PaperlessBaseURL       = os.Getenv("PAPERLESS_BASE_URL")
	PaperlessAPIToken      = os.Getenv("PAPERLESS_API_TOKEN")
	OpenaiAPIKey           = os.Getenv("OPENAI_API_KEY")
	AutoTag                = "paperless-gpt-auto"
	OcrTag                 = "paperless-gpt-ocr"
	LlmProvider            = os.Getenv("LLM_PROVIDER")
	LlmModel               = os.Getenv("LLM_MODEL")
	LogLevel               = strings.ToLower(os.Getenv("LOG_LEVEL"))
	CorrespondentBlackList = strings.Split(os.Getenv("CORRESPONDENT_BLACK_LIST"), ",")

	ForbiddenTags = []string{"paperless-gpt-auto"}

	Region = os.Getenv("AWS_REGION")
	Bucket = os.Getenv("AWS_OCR_BUCKET_NAME")
)

func init() {
	validateEnvVars()
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

	if Region == "" {
		log.Fatal("missing environment variable: AWS_REGION")
	}
	if Bucket == "" {
		log.Fatal("missing environment variable: AWS_OCR_BUCKET_NAME")
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
