# Torrent API for qBittorrent, Radarr & Sonarr

A Go API that accepts magnet links and automatically:
1. Sends them to qBittorrent with the appropriate category (`radarr` for movies or `sonarr` for TV series)
2. Adds the movie/series to Radarr or Sonarr library

## Features

- Automatic detection of movie vs TV series based on torrent name patterns
- Manual override option to specify content type
- Integration with qBittorrent Web API
- Integration with Radarr API (movies)
- Integration with Sonarr API (TV series)
- Automatic category creation in qBittorrent
- Auto-search for movie/series metadata via TMDB/TVDB

## Setup

1. Copy `.env.example` to `.env` and configure your settings:

```bash
cp .env.example .env
```

2. Edit `.env` with your service details:

```env
PORT=8080

# qBittorrent
QBITTORRENT_URL=http://localhost:8080
QBITTORRENT_USERNAME=admin
QBITTORRENT_PASSWORD=your_password

# Radarr (for movies)
RADARR_URL=http://localhost:7878
RADARR_API_KEY=your_radarr_api_key

# Sonarr (for TV series)
SONARR_URL=http://localhost:8989
SONARR_API_KEY=your_sonarr_api_key
```

You can find your Radarr/Sonarr API keys in:
- Radarr: Settings → General → API Key
- Sonarr: Settings → General → API Key

3. Install dependencies:

```bash
go mod tidy
```

4. Run the server:

```bash
go run .
```

## API Endpoints

### POST /api/torrent

Add a torrent to qBittorrent.

**Request Body:**

```json
{
  "magnet_link": "magnet:?xt=urn:btih:...",
  "type": "movie"  // Optional: "movie" or "tv". Auto-detects if not provided.
}
```

**Response:**

```json
{
  "success": true,
  "message": "Torrent added to qBittorrent and movie added to Radarr",
  "category": "radarr",
  "media_title": "Movie Title (2024)",
  "added_to_library": true
}
```

### GET /health

Health check endpoint.

## Detection Logic

The API uses pattern matching to detect content type:

### TV Series Patterns
- `S01E01`, `S1E1` - Season/Episode format
- `Season 1`, `Season 01` - Season keyword
- `1x01`, `01x01` - Alternative format
- `Complete Series`, `TV Series`
- `HDTV` releases
- `Mini-Series`

### Movie Patterns
- Year + Quality (e.g., `2024.1080p.BluRay`)
- `BluRay`, `BDRip`, `DVDRip`
- `HDRip`, `WEBRip`
- `IMAX`, `Directors Cut`, `Extended Cut`
- `CAM`, `HDCAM`, `Telesync`

## Examples

### Add a movie (auto-detect):

```bash
curl -X POST http://localhost:8080/api/torrent \
  -H "Content-Type: application/json" \
  -d '{"magnet_link": "magnet:?xt=urn:btih:abc123&dn=Movie.Name.2024.1080p.BluRay"}'
```

### Add a TV show (auto-detect):

```bash
curl -X POST http://localhost:8080/api/torrent \
  -H "Content-Type: application/json" \
  -d '{"magnet_link": "magnet:?xt=urn:btih:abc123&dn=Show.Name.S01E01.720p.HDTV"}'
```

### Force category:

```bash
curl -X POST http://localhost:8080/api/torrent \
  -H "Content-Type: application/json" \
  -d '{"magnet_link": "magnet:?xt=urn:btih:abc123", "type": "tv"}'
```

## Building

```bash
go build -o torrent-api .
```

## Docker

```bash
docker build -t torrent-api .
docker run -p 8080:8080 --env-file .env torrent-api
```
