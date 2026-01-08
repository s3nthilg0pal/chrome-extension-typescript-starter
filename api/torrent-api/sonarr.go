package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type SonarrClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type SonarrSeries struct {
	ID               int               `json:"id,omitempty"`
	Title            string            `json:"title"`
	TitleSlug        string            `json:"titleSlug"`
	Year             int               `json:"year"`
	TVDBID           int               `json:"tvdbId"`
	QualityProfileID int               `json:"qualityProfileId"`
	RootFolderPath   string            `json:"rootFolderPath"`
	Monitored        bool              `json:"monitored"`
	SeasonFolder     bool              `json:"seasonFolder"`
	SeriesType       string            `json:"seriesType"`
	AddOptions       *SonarrAddOptions `json:"addOptions,omitempty"`
}

type SonarrAddOptions struct {
	SearchForMissingEpisodes     bool   `json:"searchForMissingEpisodes"`
	SearchForCutoffUnmetEpisodes bool   `json:"searchForCutoffUnmetEpisodes"`
	Monitor                      string `json:"monitor"`
}

type SonarrSearchResult struct {
	Title     string `json:"title"`
	TitleSlug string `json:"titleSlug"`
	Year      int    `json:"year"`
	TVDBID    int    `json:"tvdbId"`
}

type SonarrRootFolder struct {
	ID   int    `json:"id"`
	Path string `json:"path"`
}

type SonarrQualityProfile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func NewSonarrClient(baseURL, apiKey string) *SonarrClient {
	return &SonarrClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *SonarrClient) doRequest(method, endpoint string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, fmt.Sprintf("%s%s", c.baseURL, endpoint), reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// SearchSeries searches for a series by term
func (c *SonarrClient) SearchSeries(term string) ([]SonarrSearchResult, error) {
	endpoint := fmt.Sprintf("/api/v3/series/lookup?term=%s", url.QueryEscape(term))
	respBody, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var results []SonarrSearchResult
	if err := json.Unmarshal(respBody, &results); err != nil {
		return nil, err
	}

	return results, nil
}

// GetRootFolders gets available root folders
func (c *SonarrClient) GetRootFolders() ([]SonarrRootFolder, error) {
	respBody, err := c.doRequest("GET", "/api/v3/rootfolder", nil)
	if err != nil {
		return nil, err
	}

	var folders []SonarrRootFolder
	if err := json.Unmarshal(respBody, &folders); err != nil {
		return nil, err
	}

	return folders, nil
}

// GetQualityProfiles gets available quality profiles
func (c *SonarrClient) GetQualityProfiles() ([]SonarrQualityProfile, error) {
	respBody, err := c.doRequest("GET", "/api/v3/qualityprofile", nil)
	if err != nil {
		return nil, err
	}

	var profiles []SonarrQualityProfile
	if err := json.Unmarshal(respBody, &profiles); err != nil {
		return nil, err
	}

	return profiles, nil
}

// AddSeries adds a series to Sonarr
func (c *SonarrClient) AddSeries(series SonarrSeries) (*SonarrSeries, error) {
	respBody, err := c.doRequest("POST", "/api/v3/series", series)
	if err != nil {
		return nil, err
	}

	var result SonarrSeries
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// AddSeriesFromMagnet extracts series info from magnet and adds to Sonarr
func (c *SonarrClient) AddSeriesFromMagnet(magnetLink string, extractedMedia *ExtractedMedia) (*SonarrSeries, error) {
	// Use extracted name from the extractor API
	searchTerm := extractedMedia.ExtractedName

	// Search for the series
	results, err := c.SearchSeries(searchTerm)
	if err != nil {
		return nil, fmt.Errorf("failed to search series: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("series not found: %s", searchTerm)
	}

	// Get first result
	searchResult := results[0]

	// Get root folder
	folders, err := c.GetRootFolders()
	if err != nil {
		return nil, fmt.Errorf("failed to get root folders: %w", err)
	}
	if len(folders) == 0 {
		return nil, fmt.Errorf("no root folders configured in Sonarr")
	}

	// Get quality profile
	profiles, err := c.GetQualityProfiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get quality profiles: %w", err)
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("no quality profiles configured in Sonarr")
	}

	// Create series
	series := SonarrSeries{
		Title:            searchResult.Title,
		TitleSlug:        searchResult.TitleSlug,
		Year:             searchResult.Year,
		TVDBID:           searchResult.TVDBID,
		QualityProfileID: profiles[0].ID,
		RootFolderPath:   folders[0].Path,
		Monitored:        true,
		SeasonFolder:     true,
		SeriesType:       "standard",
		AddOptions: &SonarrAddOptions{
			SearchForMissingEpisodes:     false, // Don't search, we're adding via torrent
			SearchForCutoffUnmetEpisodes: false,
			Monitor:                      "all",
		},
	}

	return c.AddSeries(series)
}

// AddSeriesByName searches for a series by name and adds it to Sonarr
func (c *SonarrClient) AddSeriesByName(searchTerm string) (*SonarrSeries, error) {
	// Search for the series
	results, err := c.SearchSeries(searchTerm)
	if err != nil {
		return nil, fmt.Errorf("failed to search series: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("series not found: %s", searchTerm)
	}

	// Get first result
	searchResult := results[0]

	// Get root folder
	folders, err := c.GetRootFolders()
	if err != nil {
		return nil, fmt.Errorf("failed to get root folders: %w", err)
	}
	if len(folders) == 0 {
		return nil, fmt.Errorf("no root folders configured in Sonarr")
	}

	// Get quality profile
	profiles, err := c.GetQualityProfiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get quality profiles: %w", err)
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("no quality profiles configured in Sonarr")
	}

	// Create series
	series := SonarrSeries{
		Title:            searchResult.Title,
		TitleSlug:        searchResult.TitleSlug,
		Year:             searchResult.Year,
		TVDBID:           searchResult.TVDBID,
		QualityProfileID: profiles[0].ID,
		RootFolderPath:   folders[0].Path,
		Monitored:        true,
		SeasonFolder:     true,
		SeriesType:       "standard",
		AddOptions: &SonarrAddOptions{
			SearchForMissingEpisodes:     true, // Search for episodes after adding
			SearchForCutoffUnmetEpisodes: false,
			Monitor:                      "all",
		},
	}

	return c.AddSeries(series)
}

// cleanSeriesName removes quality tags, season/episode info from torrent names
func cleanSeriesName(name string) string {
	// Remove file extension
	name = regexp.MustCompile(`\.(mkv|avi|mp4|mov|wmv)$`).ReplaceAllString(name, "")

	// Replace dots and underscores with spaces
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")

	// Remove season/episode patterns and everything after
	patterns := []string{
		`(?i)\s*S\d{1,2}E\d{1,2}.*`,        // S01E01 and everything after
		`(?i)\s*S\d{1,2}\s*-\s*E\d{1,2}.*`, // S01 - E01
		`(?i)\s*Season\s*\d+.*`,            // Season 1 and everything after
		`(?i)\s*\d{1,2}x\d{1,2}.*`,         // 1x01 and everything after
		`(?i)\s*S\d{1,2}\..*`,              // S01. and everything after
		`(?i)\s*Complete.*`,                // Complete and everything after
		`(?i)\s*(720p|1080p|2160p|4K|UHD).*`,
		`(?i)\s*(BluRay|BDRip|BRRip|DVDRip|HDRip|WEBRip|WEB-DL|HDTV).*`,
		`(?i)\s*(x264|x265|HEVC|H264|H265|XviD).*`,
		`(?i)\s*\[.*?\]`,
		`(?i)\s*\(.*?\)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		name = re.ReplaceAllString(name, "")
	}

	// Remove year (usually not needed for TV series search)
	name = regexp.MustCompile(`\s*(19|20)\d{2}\s*`).ReplaceAllString(name, " ")

	// Clean up extra spaces
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")
	name = strings.TrimSpace(name)

	return name
}
