package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

type TorrentHandler struct {
	qbClient        *QBittorrentClient
	radarrClient    *RadarrClient
	sonarrClient    *SonarrClient
	extractorClient *NameExtractorClient
}

type AddTorrentRequest struct {
	MagnetLink   string `json:"magnet_link"`
	Type         string `json:"type,omitempty"`           // "movie" or "tv" - optional, will auto-detect if not provided
	AddToLibrary bool   `json:"add_to_library,omitempty"` // Whether to add to Radarr/Sonarr library (default: true)
}

type AddTorrentResponse struct {
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	Category       string `json:"category,omitempty"`
	MediaTitle     string `json:"media_title,omitempty"`
	AddedToLibrary bool   `json:"added_to_library"`
}

type AddMediaRequest struct {
	Name string `json:"name"`           // Name of the movie or TV show
	Type string `json:"type"`           // "movie" or "tv"
	Year string `json:"year,omitempty"` // Optional year to improve search accuracy
}

type AddMediaResponse struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	MediaTitle string `json:"media_title,omitempty"`
	MediaType  string `json:"media_type,omitempty"`
	MediaID    int    `json:"media_id,omitempty"`
}

func NewTorrentHandler(qbClient *QBittorrentClient, radarrClient *RadarrClient, sonarrClient *SonarrClient, extractorClient *NameExtractorClient) *TorrentHandler {
	return &TorrentHandler{
		qbClient:        qbClient,
		radarrClient:    radarrClient,
		sonarrClient:    sonarrClient,
		extractorClient: extractorClient,
	}
}

func (h *TorrentHandler) AddTorrent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Only accept POST requests
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(AddTorrentResponse{
			Success: false,
			Message: "Method not allowed. Use POST.",
		})
		return
	}

	// Parse request body
	var req AddTorrentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AddTorrentResponse{
			Success: false,
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	// Validate magnet link
	if req.MagnetLink == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AddTorrentResponse{
			Success: false,
			Message: "Magnet link is required",
		})
		return
	}

	if !isValidMagnetLink(req.MagnetLink) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AddTorrentResponse{
			Success: false,
			Message: "Invalid magnet link format",
		})
		return
	}

	// Determine category
	var category string
	var isMovie bool
	if req.Type != "" {
		// User specified type
		switch req.Type {
		case "movie":
			category = "radarr"
			isMovie = true
		case "tv", "series":
			category = "sonarr"
			isMovie = false
		default:
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(AddTorrentResponse{
				Success: false,
				Message: "Invalid type. Use 'movie' or 'tv'",
			})
			return
		}
	} else {
		// Auto-detect type from magnet link
		category = detectCategory(req.MagnetLink)
		isMovie = category == "radarr"
	}

	log.Printf("Adding torrent with category: %s", category)

	// Extract media name using the extractor API
	torrentName := extractNameFromMagnet(req.MagnetLink)
	extractedMedia, err := h.extractorClient.ExtractName(torrentName)
	if err != nil {
		log.Printf("Warning: could not extract media name: %v", err)
		// Continue anyway, we can still add to qBittorrent
	} else {
		log.Printf("Extracted media: %s (%s) - Type: %s", extractedMedia.ExtractedName, extractedMedia.Year, extractedMedia.MediaType)

		// Use extractor's media type if user didn't specify
		if req.Type == "" && extractedMedia.MediaType != "" {
			if extractedMedia.MediaType == "movie" {
				category = "radarr"
				isMovie = true
			} else if extractedMedia.MediaType == "tv" || extractedMedia.MediaType == "series" {
				category = "sonarr"
				isMovie = false
			}
			log.Printf("Updated category based on extractor: %s", category)
		}
	}

	// Ensure category exists in qBittorrent
	if err := h.qbClient.EnsureCategory(category); err != nil {
		log.Printf("Warning: could not ensure category exists: %v", err)
	}

	// Add torrent to qBittorrent
	if err := h.qbClient.AddTorrent(req.MagnetLink, category); err != nil {
		log.Printf("Error adding torrent: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(AddTorrentResponse{
			Success: false,
			Message: "Failed to add torrent: " + err.Error(),
		})
		return
	}

	// Add to Radarr or Sonarr library
	var mediaTitle string
	addedToLibrary := false

	// Default to adding to library unless explicitly disabled
	shouldAddToLibrary := true
	// Only try to add to library if we successfully extracted the media name
	if extractedMedia == nil {
		shouldAddToLibrary = false
		log.Printf("Skipping library add - could not extract media name")
	}

	if shouldAddToLibrary {
		if isMovie {
			log.Printf("Adding movie to Radarr: %s", extractedMedia.ExtractedName)
			movie, err := h.radarrClient.AddMovieFromMagnet(req.MagnetLink, extractedMedia)
			if err != nil {
				// Check if movie already exists (common case)
				if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "exists") {
					log.Printf("Movie already exists in Radarr: %v", err)
					mediaTitle = extractedMedia.ExtractedName
					addedToLibrary = false
				} else {
					log.Printf("Warning: could not add movie to Radarr: %v", err)
					mediaTitle = extractedMedia.ExtractedName
				}
			} else {
				log.Printf("Movie added to Radarr: %s", movie.Title)
				mediaTitle = movie.Title
				addedToLibrary = true
			}
		} else {
			log.Printf("Adding series to Sonarr: %s", extractedMedia.ExtractedName)
			series, err := h.sonarrClient.AddSeriesFromMagnet(req.MagnetLink, extractedMedia)
			if err != nil {
				// Check if series already exists (common case)
				if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "exists") {
					log.Printf("Series already exists in Sonarr: %v", err)
					mediaTitle = extractedMedia.ExtractedName
					addedToLibrary = false
				} else {
					log.Printf("Warning: could not add series to Sonarr: %v", err)
					mediaTitle = extractedMedia.ExtractedName
				}
			} else {
				log.Printf("Series added to Sonarr: %s", series.Title)
				mediaTitle = series.Title
				addedToLibrary = true
			}
		}
	}

	// Success response
	message := "Torrent added to qBittorrent"
	if addedToLibrary {
		if isMovie {
			message += " and movie added to Radarr"
		} else {
			message += " and series added to Sonarr"
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(AddTorrentResponse{
		Success:        true,
		Message:        message,
		Category:       category,
		MediaTitle:     mediaTitle,
		AddedToLibrary: addedToLibrary,
	})
}

// AddMedia handles adding a movie or TV show to Radarr/Sonarr by name
func (h *TorrentHandler) AddMedia(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Only accept POST requests
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(AddMediaResponse{
			Success: false,
			Message: "Method not allowed. Use POST.",
		})
		return
	}

	// Parse request body
	var req AddMediaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AddMediaResponse{
			Success: false,
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	// Validate required fields
	if req.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AddMediaResponse{
			Success: false,
			Message: "Name is required",
		})
		return
	}

	if req.Type == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AddMediaResponse{
			Success: false,
			Message: "Type is required (movie or tv)",
		})
		return
	}

	// Normalize type
	mediaType := strings.ToLower(req.Type)
	if mediaType != "movie" && mediaType != "tv" && mediaType != "series" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(AddMediaResponse{
			Success: false,
			Message: "Invalid type. Use 'movie' or 'tv'",
		})
		return
	}

	// Build search term
	searchTerm := req.Name
	if req.Year != "" {
		searchTerm = searchTerm + " " + req.Year
	}

	log.Printf("Adding media: %s (type: %s)", searchTerm, mediaType)

	if mediaType == "movie" {
		// Add movie to Radarr
		movie, err := h.radarrClient.AddMovieByName(searchTerm)
		if err != nil {
			log.Printf("Error adding movie to Radarr: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(AddMediaResponse{
				Success: false,
				Message: "Failed to add movie: " + err.Error(),
			})
			return
		}

		log.Printf("Movie added to Radarr: %s (ID: %d)", movie.Title, movie.ID)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(AddMediaResponse{
			Success:    true,
			Message:    "Movie added to Radarr",
			MediaTitle: movie.Title,
			MediaType:  "movie",
			MediaID:    movie.ID,
		})
	} else {
		// Add series to Sonarr
		series, err := h.sonarrClient.AddSeriesByName(searchTerm)
		if err != nil {
			log.Printf("Error adding series to Sonarr: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(AddMediaResponse{
				Success: false,
				Message: "Failed to add series: " + err.Error(),
			})
			return
		}

		log.Printf("Series added to Sonarr: %s (ID: %d)", series.Title, series.ID)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(AddMediaResponse{
			Success:    true,
			Message:    "Series added to Sonarr",
			MediaTitle: series.Title,
			MediaType:  "tv",
			MediaID:    series.ID,
		})
	}
}
