package model

import "paperless-gpt/paperless/paperless_model"

// GenerateSuggestionsRequest is the request payload for generating suggestions for /generate-suggestions endpoint
type GenerateSuggestionsRequest struct {
	Documents              []paperless_model.Document `json:"documents"`
	GenerateTitles         bool                       `json:"generate_titles,omitempty"`
	GenerateTags           bool                       `json:"generate_tags,omitempty"`
	GenerateCorrespondents bool                       `json:"generate_correspondents,omitempty"`
}
