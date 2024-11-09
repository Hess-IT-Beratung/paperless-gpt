package config

import (
	_ "embed"
	"html/template"
	"os"
	"path/filepath"
)

var (

	//go:embed prompts/json_prompt.tmpl
	jsonTemplate string

	JsonPrompt *template.Template
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

	// Load json template
	jsonTemplatePath := filepath.Join(promptsDir, "json_prompt.tmpl")
	jsonTemplateContent, err := os.ReadFile(jsonTemplatePath)
	if err != nil {
		log.Infof("Could not read %s, using default template: %v", jsonTemplatePath, err)
		jsonTemplateContent = []byte(jsonTemplate)
		if err := os.WriteFile(jsonTemplatePath, jsonTemplateContent, os.ModePerm); err != nil {
			log.Fatalf("Failed to write default json template to disk: %v", err)
		}
	}
	JsonPrompt, err = template.New("json").Funcs(sprig.FuncMap()).Parse(string(jsonTemplateContent))
	if err != nil {
		log.Fatalf("Failed to parse json template: %v", err)
	}
}
