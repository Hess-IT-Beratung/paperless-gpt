package paperless_service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"paperless-gpt/internal/config"
	"paperless-gpt/internal/logging"
	"paperless-gpt/paperless/paperless_model"
	"strings"
)

var (
	log = logging.InitLogger(config.LogLevel)
)

// PaperlessClient struct to interact with the Paperless-NGX API
type PaperlessClient struct {
	BaseURL    string
	APIToken   string
	HTTPClient *http.Client
}

// NewPaperlessClient creates a new instance of PaperlessClient with a default HTTP client
func NewPaperlessClient(baseURL, apiToken string) *PaperlessClient {

	return &PaperlessClient{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		APIToken:   apiToken,
		HTTPClient: &http.Client{},
	}
}

// Do method to make requests to the Paperless-NGX API
func (paperlessClient *PaperlessClient) Do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("%s/%s", paperlessClient.BaseURL, strings.TrimLeft(path, "/"))
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Token %s", paperlessClient.APIToken))

	// Set Content-Type if body is present
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return paperlessClient.HTTPClient.Do(req)
}

// GetAllTags retrieves all tags from the Paperless-NGX API
func (paperlessClient *PaperlessClient) GetAllTags(ctx context.Context) (map[string]int, error) {
	tagIDMapping := make(map[string]int)
	path := "api/tags/"

	for path != "" {
		resp, err := paperlessClient.Do(ctx, "GET", path, nil)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("error fetching tags: %d, %s", resp.StatusCode, string(bodyBytes))
		}

		var tagsResponse struct {
			Results []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"results"`
			Next string `json:"next"`
		}

		err = json.NewDecoder(resp.Body).Decode(&tagsResponse)
		if err != nil {
			return nil, err
		}

		for _, tag := range tagsResponse.Results {
			tagIDMapping[tag.Name] = tag.ID
		}

		// Extract relative path from the Next URL
		if tagsResponse.Next != "" {
			nextURL := tagsResponse.Next
			if strings.HasPrefix(nextURL, paperlessClient.BaseURL) {
				nextURL = strings.TrimPrefix(nextURL, paperlessClient.BaseURL+"/")
			}
			path = nextURL
		} else {
			path = ""
		}
	}

	return tagIDMapping, nil
}

// GetDocumentsByTags retrieves documents that match the specified tags
func (paperlessClient *PaperlessClient) GetDocumentsByTags(ctx context.Context, tags []string, pageSize int) ([]paperless_model.Document, error) {
	tagQueries := make([]string, len(tags))
	for i, tag := range tags {
		tagQueries[i] = fmt.Sprintf("tag:%s", tag)
	}
	searchQuery := strings.Join(tagQueries, " ")
	path := fmt.Sprintf("api/documents/?query=%s&page_size=%d", urlEncode(searchQuery), pageSize)

	resp, err := paperlessClient.Do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error searching documents: %d, %s", resp.StatusCode, string(bodyBytes))
	}

	var documentsResponse paperless_model.GetDocumentsApiResponse
	err = json.NewDecoder(resp.Body).Decode(&documentsResponse)
	if err != nil {
		return nil, err
	}

	allTags, err := paperlessClient.GetAllTags(ctx)
	if err != nil {
		return nil, err
	}

	documents := make([]paperless_model.Document, 0, len(documentsResponse.Results))
	for _, result := range documentsResponse.Results {
		tagNames := make([]string, len(result.Tags))
		for i, resultTagID := range result.Tags {
			for tagName, tagID := range allTags {
				if resultTagID == tagID {
					tagNames[i] = tagName
					break
				}
			}
		}

		documents = append(documents, paperless_model.Document{
			ID:      result.ID,
			Title:   result.Title,
			Content: result.Content,
			Tags:    tagNames,
		})
	}

	return documents, nil
}

// DownloadPDF downloads the PDF file of the specified document
func (paperlessClient *PaperlessClient) DownloadPDF(ctx context.Context, document paperless_model.Document) ([]byte, error) {
	path := fmt.Sprintf("api/documents/%d/download/", document.ID)
	resp, err := paperlessClient.Do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error downloading document %d: %d, %s", document.ID, resp.StatusCode, string(bodyBytes))
	}

	return io.ReadAll(resp.Body)
}

func (paperlessClient *PaperlessClient) GetDocument(ctx context.Context, documentID int) (paperless_model.Document, error) {
	path := fmt.Sprintf("api/documents/%d/", documentID)
	resp, err := paperlessClient.Do(ctx, "GET", path, nil)
	if err != nil {
		return paperless_model.Document{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return paperless_model.Document{}, fmt.Errorf("error fetching document %d: %d, %s", documentID, resp.StatusCode, string(bodyBytes))
	}

	var documentResponse paperless_model.GetDocumentApiResponse
	err = json.NewDecoder(resp.Body).Decode(&documentResponse)
	if err != nil {
		return paperless_model.Document{}, err
	}

	allTags, err := paperlessClient.GetAllTags(ctx)
	if err != nil {
		return paperless_model.Document{}, err
	}

	tagNames := make([]string, len(documentResponse.Tags))
	for i, resultTagID := range documentResponse.Tags {
		for tagName, tagID := range allTags {
			if resultTagID == tagID {
				tagNames[i] = tagName
				break
			}
		}
	}

	return paperless_model.Document{
		ID:      documentResponse.ID,
		Title:   documentResponse.Title,
		Content: documentResponse.Content,
		Tags:    tagNames,
	}, nil
}

// UpdateDocuments updates the specified documents with suggested changes
func (paperlessClient *PaperlessClient) UpdateDocument(ctx context.Context, suggestion paperless_model.DocumentSuggestion) error {

	documentID := suggestion.DocumentID

	updatedFields := make(map[string]interface{})

	// Tags
	if suggestedTagIds, tagError := getSuggestedTags(ctx, paperlessClient, suggestion.Tags); tagError != nil {
		return tagError
	} else {
		updatedFields["tags"] = suggestedTagIds
	}

	// Correspondent
	if suggestion.Correspondent != nil {
		if suggestedCorrespondentIdPointer, correspondentError := getSuggestedCorrespondent(ctx, *suggestion.Correspondent, paperlessClient); correspondentError != nil {
			return correspondentError
		} else if suggestedCorrespondentIdPointer != nil {
			updatedFields["correspondent"] = *suggestedCorrespondentIdPointer
		}
	}

	// Document Type
	if suggestion.DocumentType != nil {
		if suggestedDocumentTypeIdPointer, documentTypeError := getSuggestedDocumentType(ctx, *suggestion.DocumentType, paperlessClient); documentTypeError != nil {
			return documentTypeError
		} else if suggestedDocumentTypeIdPointer != nil {
			updatedFields["document_type"] = *suggestedDocumentTypeIdPointer
		}
	}

	// Created Date
	if suggestion.Date != nil && len(*suggestion.Date) == len("2006-01-02") {
		updatedFields["created_date"] = suggestion.Date
	}

	// Suggested Title
	if suggestion.Title != nil {
		updatedFields["title"] = getSuggestedTitle(*suggestion.Title, suggestion.OriginalDocument.Title, documentID)
	}

	if updateError := paperlessClient.updateDocument(ctx, updatedFields, documentID); updateError != nil {
		return updateError
	}

	log.Printf("Document %d updated successfully.", documentID)
	return nil
}

func getSuggestedTags(ctx context.Context, paperlessClient *PaperlessClient, suggestedTags []string) ([]int, error) {
	suggestedTagIds := []int{}
	// Fetch all available tags
	availableTags, err := paperlessClient.GetAllTags(ctx)
	if err != nil {
		log.Errorf("Error fetching available tags: %v", err)
		return nil, err
	}

	// Map suggested tag names to IDs
	for _, tagName := range suggestedTags {
		if tagID, exists := availableTags[tagName]; exists {
			suggestedTagIds = append(suggestedTagIds, tagID)
		} else {
			log.Errorf("Suggested tag '%s' does not exist in paperless-ngx, skipping.", tagName)
		}
	}

	return suggestedTagIds, nil
}

func getSuggestedDocumentType(ctx context.Context, suggestedDocumentType string, paperlessClient *PaperlessClient) (*int, error) {
	if suggestedDocumentType == "" {
		return nil, nil
	}

	availableDocumentTypes := make(map[string]int)
	availableDocumentTypes, err := paperlessClient.GetAllDocumentTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching available document types: %v", err)
	}

	// Check if the suggested document type already exists
	if documentTypeID, exists := availableDocumentTypes[suggestedDocumentType]; exists {
		return &documentTypeID, nil
	}

	// Create a new document type if it doesn't exist
	newDocumentType := instantiateDocumentType(suggestedDocumentType)
	newDocumentTypeID, err := paperlessClient.CreateDocumentType(ctx, newDocumentType)
	if err != nil {
		return nil, fmt.Errorf("error creating document type with name %s: %v", suggestedDocumentType, err)
	}

	log.Warnf("Created document_type with name '%s' (id: %d)", suggestedDocumentType, newDocumentTypeID)
	return &newDocumentTypeID, nil
}

// instantiateDocumentType creates a new DocumentType object with default values
func instantiateDocumentType(name string) paperless_model.DocumentType {
	return paperless_model.DocumentType{
		Name:              name,
		MatchingAlgorithm: 0,
		Match:             "",
		IsInsensitive:     true,
		Owner:             nil,
	}
}

// CreateDocumentType creates a new document type in Paperless-NGX
func (paperlessClient *PaperlessClient) CreateDocumentType(ctx context.Context, documentType paperless_model.DocumentType) (int, error) {
	url := "api/document_types/"

	// Marshal the document type data to JSON
	jsonData, err := json.Marshal(documentType)
	if err != nil {
		return 0, err
	}

	// Send the POST request
	resp, err := paperlessClient.Do(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("error creating document type: %d, %s", resp.StatusCode, string(bodyBytes))
	}

	// Decode the response body to get the ID of the created document type
	var createdDocumentType struct {
		ID int `json:"id"`
	}
	err = json.NewDecoder(resp.Body).Decode(&createdDocumentType)
	if err != nil {
		return 0, err
	}

	return createdDocumentType.ID, nil
}

func getSuggestedCorrespondent(ctx context.Context, suggestedCorrespondent string, paperlessClient *PaperlessClient) (*int, error) {
	if suggestedCorrespondent == "" {
		return nil, nil
	}

	availableCorrespondents := make(map[string]int)
	availableCorrespondents, err := paperlessClient.GetAllCorrespondents(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching available correspondents: %v", err)
	}

	// Check if the suggested correspondent already exists
	if correspondentID, exists := availableCorrespondents[suggestedCorrespondent]; exists {
		return &correspondentID, nil
	}

	// Create a new correspondent if it doesn't exist
	newCorrespondent := instantiateCorrespondent(suggestedCorrespondent)
	newCorrespondentID, err := paperlessClient.CreateCorrespondent(ctx, newCorrespondent)
	if err != nil {
		return nil, fmt.Errorf("error creating correspondent with name %s: %v", suggestedCorrespondent, err)
	}

	log.Warnf("Created correspondent with name '%s'  (id: %d)", suggestedCorrespondent, newCorrespondentID)
	return &newCorrespondentID, nil
}

func getSuggestedTitle(suggestedTitle string, originalTitle string, documentID int) string {
	if len(suggestedTitle) > 128 {
		suggestedTitle = suggestedTitle[:128]
	}
	if suggestedTitle != "" {
		return suggestedTitle
	} else {
		log.Warnf("No valid title found for suggestion %d, skipping.", documentID)
		return originalTitle
	}
}

func (paperlessClient *PaperlessClient) updateDocument(ctx context.Context, updatedFields map[string]interface{}, documentID int) error {
	// Marshal updated fields to JSON
	jsonData, err := json.Marshal(updatedFields)
	if err != nil {
		log.Errorf("Error marshalling JSON for suggestion %d: %v", documentID, err)
		return err
	}

	// Send the update request
	path := fmt.Sprintf("api/documents/%d/", documentID)
	resp, err := paperlessClient.Do(ctx, "PATCH", path, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Errorf("Error updating suggestion %d: %v", documentID, err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Errorf("Error updating suggestion %d: %d, %s", documentID, resp.StatusCode, string(bodyBytes))
		return fmt.Errorf("error updating suggestion %d: %d, %s", documentID, resp.StatusCode, string(bodyBytes))
	}
	return nil
}

// urlEncode encodes a string for safe URL usage
func urlEncode(s string) string {
	return strings.ReplaceAll(s, " ", "+")
}

// instantiateCorrespondent creates a new Correspondent object with default values
func instantiateCorrespondent(name string) paperless_model.Correspondent {
	return paperless_model.Correspondent{
		Name:              name,
		MatchingAlgorithm: 0,
		Match:             "",
		IsInsensitive:     true,
		Owner:             nil,
	}
}

// CreateCorrespondent creates a new correspondent in Paperless-NGX
func (paperlessClient *PaperlessClient) CreateCorrespondent(ctx context.Context, correspondent paperless_model.Correspondent) (int, error) {
	url := "api/correspondents/"

	// Marshal the correspondent data to JSON
	jsonData, err := json.Marshal(correspondent)
	if err != nil {
		return 0, err
	}

	// Send the POST request
	resp, err := paperlessClient.Do(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("error creating correspondent: %d, %s", resp.StatusCode, string(bodyBytes))
	}

	// Decode the response body to get the DocumentID of the created correspondent
	var createdCorrespondent struct {
		ID int `json:"id"`
	}
	err = json.NewDecoder(resp.Body).Decode(&createdCorrespondent)
	if err != nil {
		return 0, err
	}

	return createdCorrespondent.ID, nil
}

// CorrespondentResponse represents the response structure for correspondents
type CorrespondentResponse struct {
	Results []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"results"`
}

// GetAllDocumentTypes retrieves all document types from the Paperless-NGX API
func (paperlessClient *PaperlessClient) GetAllDocumentTypes(ctx context.Context) (map[string]int, error) {
	documentTypeIDMapping := make(map[string]int)
	path := "api/document_types/?page_size=9999"

	resp, err := paperlessClient.Do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error fetching document types: %d, %s", resp.StatusCode, string(bodyBytes))
	}

	var documentTypesResponse struct {
		Results []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"results"`
	}

	err = json.NewDecoder(resp.Body).Decode(&documentTypesResponse)
	if err != nil {
		return nil, err
	}

	for _, documentType := range documentTypesResponse.Results {
		documentTypeIDMapping[documentType.Name] = documentType.ID
	}

	return documentTypeIDMapping, nil
}

// GetAllCorrespondents retrieves all correspondents from the Paperless-NGX API
func (paperlessClient *PaperlessClient) GetAllCorrespondents(ctx context.Context) (map[string]int, error) {
	correspondentIDMapping := make(map[string]int)
	path := "api/correspondents/?page_size=9999"

	resp, err := paperlessClient.Do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error fetching correspondents: %d, %s", resp.StatusCode, string(bodyBytes))
	}

	var correspondentsResponse CorrespondentResponse

	err = json.NewDecoder(resp.Body).Decode(&correspondentsResponse)
	if err != nil {
		return nil, err
	}

	for _, correspondent := range correspondentsResponse.Results {
		correspondentIDMapping[correspondent.Name] = correspondent.ID
	}

	return correspondentIDMapping, nil
}

// RemoveTagFromList removes a specific tag from a list of tags
func RemoveTagFromList(tags []string, tagToRemove string) []string {
	filteredTags := []string{}
	for _, tag := range tags {
		if tag != tagToRemove {
			filteredTags = append(filteredTags, tag)
		}
	}
	return filteredTags
}
