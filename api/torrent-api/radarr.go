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

type RadarrClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type RadarrMovie struct {
	ID                  int               `json:"id,omitempty"`
	Title               string            `json:"title"`
	TitleSlug           string            `json:"titleSlug"`
	Year                int               `json:"year"`
	TMDBID              int               `json:"tmdbId"`
	QualityProfileID    int               `json:"qualityProfileId"`
	RootFolderPath      string            `json:"rootFolderPath"`
	Monitored           bool              `json:"monitored"`
	MinimumAvailability string            `json:"minimumAvailability"`
	AddOptions          *RadarrAddOptions `json:"addOptions,omitempty"`
}

type RadarrAddOptions struct {
	SearchForMovie bool `json:"searchForMovie"`
}

type RadarrSearchResult struct {
	Title     string `json:"title"`
	TitleSlug string `json:"titleSlug"`
	Year      int    `json:"year"`
	TMDBID    int    `json:"tmdbId"`
}

type RadarrRootFolder struct {
	ID   int    `json:"id"`
	Path string `json:"path"`
}

type RadarrQualityProfile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func NewRadarrClient(baseURL, apiKey string) *RadarrClient {
	return &RadarrClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *RadarrClient) doRequest(method, endpoint string, body interface{}) ([]byte, error) {
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

// SearchMovie searches for a movie by term
func (c *RadarrClient) SearchMovie(term string) ([]RadarrSearchResult, error) {
	endpoint := fmt.Sprintf("/api/v3/movie/lookup?term=%s", url.QueryEscape(term))
	respBody, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var results []RadarrSearchResult
	if err := json.Unmarshal(respBody, &results); err != nil {
		return nil, err
	}

	return results, nil
}

// GetRootFolders gets available root folders
func (c *RadarrClient) GetRootFolders() ([]RadarrRootFolder, error) {
	respBody, err := c.doRequest("GET", "/api/v3/rootfolder", nil)
	if err != nil {
		return nil, err
	}

	var folders []RadarrRootFolder
	if err := json.Unmarshal(respBody, &folders); err != nil {
		return nil, err
	}

	return folders, nil
}

// GetQualityProfiles gets available quality profiles
func (c *RadarrClient) GetQualityProfiles() ([]RadarrQualityProfile, error) {
	respBody, err := c.doRequest("GET", "/api/v3/qualityprofile", nil)
	if err != nil {
		return nil, err
	}

	var profiles []RadarrQualityProfile
	if err := json.Unmarshal(respBody, &profiles); err != nil {
		return nil, err
	}

	return profiles, nil
}

// AddMovie adds a movie to Radarr
func (c *RadarrClient) AddMovie(movie RadarrMovie) (*RadarrMovie, error) {
	respBody, err := c.doRequest("POST", "/api/v3/movie", movie)
	if err != nil {
		return nil, err
	}

	var result RadarrMovie
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// AddMovieFromMagnet extracts movie info from magnet and adds to Radarr
func (c *RadarrClient) AddMovieFromMagnet(magnetLink string, extractedMedia *ExtractedMedia) (*RadarrMovie, error) {
	// Use extracted name from the extractor API
	searchTerm := extractedMedia.ExtractedName
	if extractedMedia.Year != "" {
		searchTerm = searchTerm + " " + extractedMedia.Year
	}

	// Search for the movie
	results, err := c.SearchMovie(searchTerm)
	if err != nil {
		return nil, fmt.Errorf("failed to search movie: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("movie not found: %s", searchTerm)
	}

	// Get first result
	searchResult := results[0]

	// Get root folder
	folders, err := c.GetRootFolders()
	if err != nil {
		return nil, fmt.Errorf("failed to get root folders: %w", err)
	}
	if len(folders) == 0 {
		return nil, fmt.Errorf("no root folders configured in Radarr")
	}

	// Get quality profile
	profiles, err := c.GetQualityProfiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get quality profiles: %w", err)
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("no quality profiles configured in Radarr")
	}

	// Create movie
	movie := RadarrMovie{
		Title:               searchResult.Title,
		TitleSlug:           searchResult.TitleSlug,
		Year:                searchResult.Year,
		TMDBID:              searchResult.TMDBID,
		QualityProfileID:    profiles[0].ID,
		RootFolderPath:      folders[0].Path,
		Monitored:           true,
		MinimumAvailability: "released",
		AddOptions: &RadarrAddOptions{
			SearchForMovie: false, // Don't search, we're adding via torrent
		},
	}

	return c.AddMovie(movie)
}

// AddMovieByName searches for a movie by name and adds it to Radarr
func (c *RadarrClient) AddMovieByName(searchTerm string) (*RadarrMovie, error) {
	// Search for the movie
	results, err := c.SearchMovie(searchTerm)
	if err != nil {
		return nil, fmt.Errorf("failed to search movie: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("movie not found: %s", searchTerm)
	}

	// Get first result
	searchResult := results[0]

	// Get root folder
	folders, err := c.GetRootFolders()
	if err != nil {
		return nil, fmt.Errorf("failed to get root folders: %w", err)
	}
	if len(folders) == 0 {
		return nil, fmt.Errorf("no root folders configured in Radarr")
	}

	// Get quality profile
	profiles, err := c.GetQualityProfiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get quality profiles: %w", err)
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("no quality profiles configured in Radarr")
	}

	// Create movie
	movie := RadarrMovie{
		Title:               searchResult.Title,
		TitleSlug:           searchResult.TitleSlug,
		Year:                searchResult.Year,
		TMDBID:              searchResult.TMDBID,
		QualityProfileID:    profiles[0].ID,
		RootFolderPath:      folders[0].Path,
		Monitored:           true,
		MinimumAvailability: "released",
		AddOptions: &RadarrAddOptions{
			SearchForMovie: true, // Search for the movie after adding
		},
	}

	return c.AddMovie(movie)
}

// cleanTorrentName removes quality tags and other noise from torrent names to extract movie title
func cleanTorrentName(name string) string {
	// Remove file extension
	name = regexp.MustCompile(`(?i)\.(mkv|avi|mp4|mov|wmv|m4v|flv|webm)$`).ReplaceAllString(name, "")

	// Replace dots, underscores, and dashes with spaces (but preserve dashes in words)
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")

	// Extract year first (we'll need it for the search)
	yearPattern := regexp.MustCompile(`[\s\(\[]((?:19|20)\d{2})[\s\)\]]?`)
	yearMatches := yearPattern.FindStringSubmatch(name)
	year := ""
	if len(yearMatches) > 1 {
		year = yearMatches[1]
	}

	// Patterns that indicate the start of release info (cut everything after)
	cutoffPatterns := []string{
		// Quality indicators
		`(?i)\b(720p|1080p|2160p|4K|UHD|HD|SD)\b.*`,
		// Source indicators
		`(?i)\b(BluRay|Blu-Ray|BDRip|BRRip|DVDRip|DVDR|DVD-R|HDRip|WEBRip|WEB-DL|WEBDL|WEB|HDTV|HDR|SDR|CAM|HDCAM|TS|TELESYNC|TC|TELECINE|SCR|SCREENER|R5|DVDScr)\b.*`,
		// Codec indicators
		`(?i)\b(x264|x265|HEVC|H\.?264|H\.?265|XviD|DivX|AVC|MPEG|VP9|AV1)\b.*`,
		// Audio indicators
		`(?i)\b(AAC|AC3|DTS|DTS-HD|TrueHD|Atmos|FLAC|MP3|DD5\.?1|DD7\.?1|5\.1|7\.1)\b.*`,
		// Release groups and tags
		`(?i)\b(YIFY|YTS|RARBG|SPARKS|AXXO|FGT|EVO|GECKOS|DRONES|STUTTERSHIT|PSA|MkvCage|ETRG|EtHD|VPPV|ION10|BONE|NTG|CMRG|FLUX|NOGRP)\b.*`,
		// Other common tags
		`(?i)\b(EXTENDED|UNRATED|DIRECTORS\.?CUT|DC|THEATRICAL|REMASTERED|IMAX|3D|PROPER|REPACK|INTERNAL|LIMITED|COMPLETE|FINAL)\b.*`,
		// Language tags
		`(?i)\b(MULTI|MULTi|DUAL|FRENCH|GERMAN|SPANISH|ITALIAN|RUSSIAN|HINDI|KOREAN|JAPANESE|CHINESE)\b.*`,
		// Subtitles
		`(?i)\b(SUBBED|DUBBED|SUBS|HARDSUB|HARDCODED|HC)\b.*`,
	}

	for _, pattern := range cutoffPatterns {
		re := regexp.MustCompile(pattern)
		name = re.ReplaceAllString(name, "")
	}

	// Remove bracketed content (usually contains release info)
	name = regexp.MustCompile(`\[.*?\]`).ReplaceAllString(name, "")
	name = regexp.MustCompile(`\{.*?\}`).ReplaceAllString(name, "")

	// Remove parenthesized content (Go's regexp doesn't support lookahead, so we remove all and rely on year extraction above)
	name = regexp.MustCompile(`\([^)]*\)`).ReplaceAllString(name, "")

	// Remove standalone year (we'll add it back at the end)
	name = regexp.MustCompile(`\b(19|20)\d{2}\b`).ReplaceAllString(name, "")

	// Remove common prefixes/suffixes
	name = regexp.MustCompile(`(?i)^(www\.[^\s]+\s*-?\s*)`).ReplaceAllString(name, "")
	name = regexp.MustCompile(`(?i)(-?\s*www\.[^\s]+)$`).ReplaceAllString(name, "")

	// Remove torrent site names
	name = regexp.MustCompile(`(?i)\b(tamilrockers|tamilmv|tamilblasters|tamilyogi|isaimini|movierulz|filmyzilla|bolly4u|khatrimaza|123movies|putlocker|fmovies|gomovies|primewire|solarmovie|yesmovies|cmovies|bmovies|azmovies|lookmovie|flixtor|hdeuropix|soap2day|bflix|m4uhd|hdtoday|myflixer|dopebox|sockshare|vumoo|1337x|kickass|piratebay|rartv|ettv|eztv)\b\s*-?\s*`).ReplaceAllString(name, "")

	// Remove site URLs and patterns like [TamilMV] or - TamilRockers
	name = regexp.MustCompile(`(?i)\[\s*(tamilrockers|tamilmv|tamilblasters|tamilyogi)\s*\]`).ReplaceAllString(name, "")
	name = regexp.MustCompile(`(?i)-\s*(tamilrockers|tamilmv|tamilblasters|tamilyogi)\s*$`).ReplaceAllString(name, "")

	// Clean up extra spaces and dashes
	name = regexp.MustCompile(`\s*-\s*$`).ReplaceAllString(name, "")
	name = regexp.MustCompile(`^\s*-\s*`).ReplaceAllString(name, "")
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")
	name = strings.TrimSpace(name)

	// Add year back for better search results
	if year != "" {
		name = name + " " + year
	}

	return name
}

// ExtractMovieInfo extracts structured movie information from a torrent name
type MovieInfo struct {
	Title   string
	Year    string
	Quality string
	Source  string
	Codec   string
	Audio   string
	Group   string
}

func ExtractMovieInfo(torrentName string) MovieInfo {
	info := MovieInfo{}
	name := torrentName

	// Remove file extension
	name = regexp.MustCompile(`(?i)\.(mkv|avi|mp4|mov|wmv|m4v)$`).ReplaceAllString(name, "")

	// Replace separators with spaces for easier parsing
	workingName := strings.ReplaceAll(name, ".", " ")
	workingName = strings.ReplaceAll(workingName, "_", " ")

	// Extract year
	yearPattern := regexp.MustCompile(`\b((?:19|20)\d{2})\b`)
	if matches := yearPattern.FindStringSubmatch(workingName); len(matches) > 1 {
		info.Year = matches[1]
	}

	// Extract quality
	qualityPattern := regexp.MustCompile(`(?i)\b(720p|1080p|2160p|4K|UHD)\b`)
	if matches := qualityPattern.FindStringSubmatch(workingName); len(matches) > 1 {
		info.Quality = strings.ToUpper(matches[1])
	}

	// Extract source
	sourcePattern := regexp.MustCompile(`(?i)\b(BluRay|Blu-Ray|BDRip|BRRip|DVDRip|DVDR|HDRip|WEBRip|WEB-DL|WEBDL|WEB|HDTV|CAM|HDCAM|TS|TELESYNC)\b`)
	if matches := sourcePattern.FindStringSubmatch(workingName); len(matches) > 1 {
		info.Source = matches[1]
	}

	// Extract codec
	codecPattern := regexp.MustCompile(`(?i)\b(x264|x265|HEVC|H\.?264|H\.?265|XviD|AVC)\b`)
	if matches := codecPattern.FindStringSubmatch(workingName); len(matches) > 1 {
		info.Codec = matches[1]
	}

	// Extract audio
	audioPattern := regexp.MustCompile(`(?i)\b(AAC|AC3|DTS|DTS-HD|TrueHD|Atmos|FLAC|DD5\.?1|DD7\.?1)\b`)
	if matches := audioPattern.FindStringSubmatch(workingName); len(matches) > 1 {
		info.Audio = matches[1]
	}

	// Extract release group (usually at the end after a dash)
	groupPattern := regexp.MustCompile(`-([A-Za-z0-9]+)(?:\s*\[.*\])?$`)
	if matches := groupPattern.FindStringSubmatch(name); len(matches) > 1 {
		// Make sure it's not a quality/codec tag
		group := matches[1]
		if !regexp.MustCompile(`(?i)^(720p|1080p|2160p|x264|x265|HEVC|AAC|AC3|DTS)$`).MatchString(group) {
			info.Group = group
		}
	}

	// Extract title (everything before year or quality indicators)
	info.Title = cleanTorrentName(torrentName)

	return info
}
