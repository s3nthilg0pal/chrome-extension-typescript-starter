package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type NameExtractorClient struct {
	baseURL    string
	httpClient *http.Client
}

type ExtractedMedia struct {
	OriginalInput string `json:"original_input"`
	ExtractedName string `json:"extracted_name"`
	Year          string `json:"year"`
	MediaType     string `json:"media_type"`
}

func NewNameExtractorClient(baseURL string) *NameExtractorClient {
	return &NameExtractorClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ExtractName calls the external API to extract movie/series name from torrent name
func (c *NameExtractorClient) ExtractName(torrentName string) (*ExtractedMedia, error) {
	endpoint := fmt.Sprintf("%s/extract?q=%s", c.baseURL, url.QueryEscape(torrentName))

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to call name extractor API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("name extractor API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result ExtractedMedia
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}
