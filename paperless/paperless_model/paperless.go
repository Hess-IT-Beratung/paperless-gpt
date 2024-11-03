package paperless_model

import (
	"time"
)

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
	DocumentID       int      `json:"id"`
	OriginalDocument Document `json:"original_document"`
	Correspondent    *string  `json:"correspondent,omitempty"`
	Title            *string  `json:"title,omitempty"`
	Date             *string  `json:"created_date,omitempty"`
	Tags             []string `json:"tags"`
	DocumentType     *string  `json:"document_type,omitempty"`
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

// Document is a stripped down version of the document object from paperless-ngx.
// Response payload for /documents endpoint and part of request payload for /generate-suggestions endpoint
type Document struct {
	ID      int      `json:"id"`
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
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
		Created             time.Time     `json:"created"`
		CreatedDate         string        `json:"created_date"`
		Modified            time.Time     `json:"modified"`
		Added               time.Time     `json:"added"`
		ArchiveSerialNumber interface{}   `json:"archive_serial_number"`
		OriginalFileName    string        `json:"original_file_name"`
		ArchivedFileName    string        `json:"archived_file_name"`
		Owner               int           `json:"owner"`
		UserCanChange       bool          `json:"user_can_change"`
		Notes               []interface{} `json:"notes"`
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
	Created             time.Time     `json:"created"`
	CreatedDate         string        `json:"created_date"`
	Modified            time.Time     `json:"modified"`
	Added               time.Time     `json:"added"`
	ArchiveSerialNumber interface{}   `json:"archive_serial_number"`
	OriginalFileName    string        `json:"original_file_name"`
	ArchivedFileName    string        `json:"archived_file_name"`
	Owner               int           `json:"owner"`
	UserCanChange       bool          `json:"user_can_change"`
	Notes               []interface{} `json:"notes"`
}
