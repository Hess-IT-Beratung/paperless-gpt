package config

import (
	_ "embed"
	"html/template"
	"os"
	"path/filepath"

	"github.com/Masterminds/sprig/v3"
)

var (
	//go:embed prompts/correspondent_prompt.tmpl
	correspondentTemplate string

	//go:embed prompts/tag_prompt.tmpl
	tagTemplate string

	//go:embed prompts/title_prompt.tmpl
	titleTemplate string

	// Templates
	TitlePrompt         *template.Template
	TagPrompt           *template.Template
	CorrespondentPrompt *template.Template
)

// loadTemplates loads the title and tag templates from files or uses default templates
func init() {

	// Ensure prompts directory exists
	promptsDir := os.Getenv("PROMPTS_DIR")

	if promptsDir == "" {
		log.Fatalf("Please set the PROMPTS_DIR environment variable.")
	}

	if _, err := os.Stat(promptsDir); os.IsNotExist(err) {
		log.Fatalf("Prompts directory does not exist: %s", promptsDir)
	}

	if err := os.MkdirAll(promptsDir, os.ModePerm); err != nil {
		log.Fatalf("Failed to create prompts directory: %v", err)
	}

	// Load title template
	titleTemplatePath := filepath.Join(promptsDir, "title_prompt.tmpl")
	titleTemplateContent, err := os.ReadFile(titleTemplatePath)
	if err != nil {
		log.Errorf("Could not read %s, using default template: %v", titleTemplatePath, err)
		titleTemplateContent = []byte(titleTemplate)
		if err := os.WriteFile(titleTemplatePath, titleTemplateContent, os.ModePerm); err != nil {
			log.Fatalf("Failed to write default title template to disk: %v", err)
		}
	}
	TitlePrompt, err = template.New("title").Funcs(sprig.FuncMap()).Parse(string(titleTemplateContent))
	if err != nil {
		log.Fatalf("Failed to parse title template: %v", err)
	}

	// Load tag template
	tagTemplatePath := filepath.Join(promptsDir, "tag_prompt.tmpl")
	tagTemplateContent, err := os.ReadFile(tagTemplatePath)
	if err != nil {
		log.Errorf("Could not read %s, using default template: %v", tagTemplatePath, err)
		tagTemplateContent = []byte(tagTemplate)
		if err := os.WriteFile(tagTemplatePath, tagTemplateContent, os.ModePerm); err != nil {
			log.Fatalf("Failed to write default tag template to disk: %v", err)
		}
	}
	TagPrompt, err = template.New("tag").Funcs(sprig.FuncMap()).Parse(string(tagTemplateContent))
	if err != nil {
		log.Fatalf("Failed to parse tag template: %v", err)
	}

	// Load correspondent template
	correspondentTemplatePath := filepath.Join(promptsDir, "correspondent_prompt.tmpl")
	correspondentTemplateContent, err := os.ReadFile(correspondentTemplatePath)
	if err != nil {
		log.Errorf("Could not read %s, using default template: %v", correspondentTemplatePath, err)
		correspondentTemplateContent = []byte(correspondentTemplate)
		if err := os.WriteFile(correspondentTemplatePath, correspondentTemplateContent, os.ModePerm); err != nil {
			log.Fatalf("Failed to write default correspondent template to disk: %v", err)
		}
	}
	CorrespondentPrompt, err = template.New("correspondent").Funcs(sprig.FuncMap()).Parse(string(correspondentTemplateContent))
	if err != nil {
		log.Fatalf("Failed to parse correspondent template: %v", err)
	}

}
