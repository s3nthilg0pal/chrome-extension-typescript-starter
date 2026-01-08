package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists
	godotenv.Load()

	// Get configuration from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize qBittorrent client
	qbClient := NewQBittorrentClient(
		os.Getenv("QBITTORRENT_URL"),
		os.Getenv("QBITTORRENT_USERNAME"),
		os.Getenv("QBITTORRENT_PASSWORD"),
	)

	// Initialize Radarr client
	radarrClient := NewRadarrClient(
		os.Getenv("RADARR_URL"),
		os.Getenv("RADARR_API_KEY"),
	)

	// Initialize Sonarr client
	sonarrClient := NewSonarrClient(
		os.Getenv("SONARR_URL"),
		os.Getenv("SONARR_API_KEY"),
	)

	// Initialize name extractor client
	extractorURL := os.Getenv("NAME_EXTRACTOR_URL")
	if extractorURL == "" {
		extractorURL = "http://localhost:8000"
	}
	extractorClient := NewNameExtractorClient(extractorURL)

	// Create handler
	handler := NewTorrentHandler(qbClient, radarrClient, sonarrClient, extractorClient)

	// Setup routes
	http.HandleFunc("/api/torrent", handler.AddTorrent)
	http.HandleFunc("/api/media", handler.AddMedia)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
