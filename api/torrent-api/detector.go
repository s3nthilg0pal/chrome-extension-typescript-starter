package main

import (
	"net/url"
	"regexp"
	"strings"
)

// TV show patterns - these indicate a TV series
var tvPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)S\d{1,2}E\d{1,2}`),        // S01E01, S1E1
	regexp.MustCompile(`(?i)S\d{1,2}\s*-\s*E\d{1,2}`), // S01 - E01
	regexp.MustCompile(`(?i)Season\s*\d+`),            // Season 1, Season 01
	regexp.MustCompile(`(?i)Episode\s*\d+`),           // Episode 1
	regexp.MustCompile(`(?i)\d{1,2}x\d{1,2}`),         // 1x01, 01x01
	regexp.MustCompile(`(?i)\.S\d{1,2}\.`),            // .S01.
	regexp.MustCompile(`(?i)Complete\s*Series`),       // Complete Series
	regexp.MustCompile(`(?i)TV\s*Series`),             // TV Series
	regexp.MustCompile(`(?i)HDTV`),                    // HDTV (usually TV shows)
	regexp.MustCompile(`(?i)WEB-?DL.*S\d{1,2}`),       // WEBDL with season
	regexp.MustCompile(`(?i)Season\s*\d+.*Complete`),  // Season X Complete
	regexp.MustCompile(`(?i)\[?\d{1,2}of\d{1,2}\]?`),  // 1of10, [1of10]
	regexp.MustCompile(`(?i)E\d{2,4}`),                // E01, E001 (episode only)
	regexp.MustCompile(`(?i)Part\s*\d+\s*of\s*\d+`),   // Part 1 of 10
	regexp.MustCompile(`(?i)S\d{1,2}\.Complete`),      // S01.Complete
	regexp.MustCompile(`(?i)Mini[.-]?Series`),         // Mini-Series
}

// Movie patterns - these indicate a movie
var moviePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(19|20)\d{2}.*?(720p|1080p|2160p|4K|BluRay|BDRip|HDRip|WEBRip|DVDR)`), // Year + quality
	regexp.MustCompile(`(?i)BluRay`),           // BluRay release
	regexp.MustCompile(`(?i)BDRip`),            // BDRip release
	regexp.MustCompile(`(?i)DVDRip`),           // DVDRip release
	regexp.MustCompile(`(?i)DVDR`),             // DVDR release
	regexp.MustCompile(`(?i)CAM\b`),            // CAM release
	regexp.MustCompile(`(?i)HDCAM`),            // HDCAM release
	regexp.MustCompile(`(?i)TS\b`),             // Telesync
	regexp.MustCompile(`(?i)TELESYNC`),         // Telesync
	regexp.MustCompile(`(?i)HDRip`),            // HDRip
	regexp.MustCompile(`(?i)WEB-?Rip`),         // WEBRip (without season indicator)
	regexp.MustCompile(`(?i)IMAX`),             // IMAX
	regexp.MustCompile(`(?i)Directors?\.?Cut`), // Director's Cut
	regexp.MustCompile(`(?i)Extended\.?Cut`),   // Extended Cut
	regexp.MustCompile(`(?i)Unrated`),          // Unrated
	regexp.MustCompile(`(?i)Theatrical`),       // Theatrical
}

// extractNameFromMagnet extracts the display name from a magnet link
func extractNameFromMagnet(magnetLink string) string {
	// Parse the magnet URI
	u, err := url.Parse(magnetLink)
	if err != nil {
		return magnetLink
	}

	// Get the 'dn' (display name) parameter
	params := u.Query()
	dn := params.Get("dn")
	if dn != "" {
		// URL decode the display name
		decoded, err := url.QueryUnescape(dn)
		if err == nil {
			return decoded
		}
		return dn
	}

	return magnetLink
}

// detectCategory analyzes the magnet link and determines if it's a movie or TV show
func detectCategory(magnetLink string) string {
	name := extractNameFromMagnet(magnetLink)
	name = strings.ToLower(name)

	// First check for TV patterns (more specific)
	tvScore := 0
	for _, pattern := range tvPatterns {
		if pattern.MatchString(name) {
			tvScore++
		}
	}

	// Then check for movie patterns
	movieScore := 0
	for _, pattern := range moviePatterns {
		if pattern.MatchString(name) {
			movieScore++
		}
	}

	// If we have strong TV indicators, it's likely a TV show
	// TV patterns like S01E01 are very specific
	if tvScore > 0 {
		// Check if it has a season/episode pattern which is definitive
		seasonEpisode := regexp.MustCompile(`(?i)S\d{1,2}E\d{1,2}`)
		if seasonEpisode.MatchString(name) {
			return "sonarr"
		}
		// Season pattern is also very indicative
		seasonPattern := regexp.MustCompile(`(?i)(Season\s*\d+|\.S\d{1,2}\.)`)
		if seasonPattern.MatchString(name) {
			return "sonarr"
		}
	}

	// Compare scores
	if tvScore > movieScore {
		return "sonarr"
	}
	if movieScore > tvScore {
		return "radarr"
	}

	// If we can't determine, default to radarr (movies)
	// This is because most single releases without season indicators are movies
	return "radarr"
}

// isValidMagnetLink checks if the string is a valid magnet link
func isValidMagnetLink(link string) bool {
	return strings.HasPrefix(strings.ToLower(link), "magnet:?")
}
