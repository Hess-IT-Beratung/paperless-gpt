package paperless_model

import (
	"encoding/json"
	"strings"
	"time"
)

// FlexibleTime handles both RFC3339 timestamps and date-only strings
type FlexibleTime struct {
	time.Time
}

func (ft *FlexibleTime) UnmarshalJSON(data []byte) error {
	str := strings.Trim(string(data), `"`)
	
	// Try RFC3339 format first (with time)
	if t, err := time.Parse(time.RFC3339, str); err == nil {
		ft.Time = t
		return nil
	}
	
	// Try date-only format
	if t, err := time.Parse("2006-01-02", str); err == nil {
		ft.Time = t
		return nil
	}
	
	// Try other common formats
	if t, err := time.Parse("2006-01-02T15:04:05", str); err == nil {
		ft.Time = t
		return nil
	}
	
	return json.Unmarshal(data, &ft.Time)
}

type DocumentType struct {
	Name              string `json:"name"`
	MatchingAlgorithm int    `json:"matching_algorithm"`
	Match             string `json:"match"`
	IsInsensitive     bool   `json:"is_insensitive"`
	Owner             *int   `json:"owner"`
	SetPermissions    struct {
		View struct {
			Users  []int `json:"users"`
			Groups []int `json:"groups"`
		} `json:"view"`
		Change struct {
			Users  []int `json:"users"`
			Groups []int `json:"groups"`
		} `json:"change"`
	} `json:"set_permissions"`
}

// DocumentSuggestion is the response payload for /generate-suggestions endpoint and the request payload for /update-documents endpoint (as an array)
type DocumentSuggestion struct {
	DocumentID       int       `json:"id"`
	OriginalDocument Document  `json:"original_document"`
	Correspondent    *string   `json:"correspondent,omitempty"`
	Title            *string   `json:"title,omitempty"`
	Date             *string   `json:"created_date,omitempty"`
	Tags             *[]string `json:"tags"`
	DocumentType     *string   `json:"document_type,omitempty"`
	Content          *string   `json:"content,omitempty"`
}

type Correspondent struct {
	Name              string `json:"name"`
	MatchingAlgorithm int    `json:"matching_algorithm"`
	Match             string `json:"match"`
	IsInsensitive     bool   `json:"is_insensitive"`
	Owner             *int   `json:"owner"`
	SetPermissions    struct {
		View struct {
			Users  []int `json:"users"`
			Groups []int `json:"groups"`
		} `json:"view"`
		Change struct {
			Users  []int `json:"users"`
			Groups []int `json:"groups"`
		} `json:"change"`
	} `json:"set_permissions"`
}

type CustomField struct {
	Value *string `json:"value"`
	Field int     `json:"field"`
}

type Document struct {
	ID               int           `json:"id"`
	Title            string        `json:"title"`
	Content          string        `json:"content"`
	Tags             []string      `json:"tags"`
	OriginalFileName string        `json:"original_file_name"`
	CustomFields     []CustomField `json:"custom_fields"`
}

type GetDocumentsApiResponse struct {
	Count    int         `json:"count"`
	Next     interface{} `json:"next"`
	Previous interface{} `json:"previous"`
	All      []int       `json:"all"`
	Results  []struct {
		ID                  int           `json:"id"`
		Correspondent       interface{}   `json:"correspondent"`
		DocumentType        interface{}   `json:"document_type"`
		StoragePath         interface{}   `json:"storage_path"`
		Title               string        `json:"title"`
		Content             string        `json:"content"`
		Tags                []int         `json:"tags"`
		Created             FlexibleTime  `json:"created"`
		CreatedDate         string        `json:"created_date"`
		Modified            FlexibleTime  `json:"modified"`
		Added               FlexibleTime  `json:"added"`
		ArchiveSerialNumber interface{}   `json:"archive_serial_number"`
		OriginalFileName    string        `json:"original_file_name"`
		ArchivedFileName    string        `json:"archived_file_name"`
		Owner               int           `json:"owner"`
		UserCanChange       bool          `json:"user_can_change"`
		Notes               []interface{} `json:"notes"`
		CustomFields        []CustomField `json:"custom_fields"`
		SearchHit           struct {
			Score          float64 `json:"score"`
			Highlights     string  `json:"highlights"`
			NoteHighlights string  `json:"note_highlights"`
			Rank           int     `json:"rank"`
		} `json:"__search_hit__"`
	} `json:"results"`
}

type GetDocumentApiResponse struct {
	ID                  int           `json:"id"`
	Correspondent       interface{}   `json:"correspondent"`
	DocumentType        interface{}   `json:"document_type"`
	StoragePath         interface{}   `json:"storage_path"`
	Title               string        `json:"title"`
	Content             string        `json:"content"`
	Tags                []int         `json:"tags"`
	Created             FlexibleTime  `json:"created"`
	CreatedDate         string        `json:"created_date"`
	Modified            FlexibleTime  `json:"modified"`
	Added               FlexibleTime  `json:"added"`
	ArchiveSerialNumber interface{}   `json:"archive_serial_number"`
	OriginalFileName    string        `json:"original_file_name"`
	ArchivedFileName    string        `json:"archived_file_name"`
	Owner               int           `json:"owner"`
	UserCanChange       bool          `json:"user_can_change"`
	Notes               []interface{} `json:"notes"`
	CustomFields        []CustomField `json:"custom_fields"`
}
