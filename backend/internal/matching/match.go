package matching

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// MatchThreshold is the minimum score to treat two tracks as the same work.
const MatchThreshold = 0.85

// Platform identifies which music service a track belongs to.
type Platform string

const (
	PlatformSpotify Platform = "spotify"
	PlatformYouTube Platform = "youtube"
)

// Track holds platform-native metadata. ID is never compared across platforms.
type Track struct {
	Platform Platform `json:"platform"`
	ID       string   `json:"id,omitempty"`
	Title    string   `json:"title"`
	Artist   string   `json:"artist,omitempty"`
	RawTitle string   `json:"raw_title,omitempty"`
}

// Decision captures whether a source track exists in a destination playlist.
type Decision struct {
	Source     Track   `json:"source"`
	BestMatch  *Track  `json:"best_match,omitempty"`
	Score      float64 `json:"score"`
	Method     string  `json:"method"`
	Matched    bool    `json:"matched"`
	QueuedAdd  bool    `json:"queued_add,omitempty"`
	SkipReason string  `json:"skip_reason,omitempty"`
}

var noisePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\(official\s+(video|audio|lyric\s*video|music\s*video)\)`),
	regexp.MustCompile(`(?i)\[official\s+(video|audio)\]`),
	regexp.MustCompile(`(?i)\(visualizer\)`),
	regexp.MustCompile(`(?i)\(hd\)`),
	regexp.MustCompile(`(?i)\(4k\)`),
}

// YouTubeFromRaw builds a Track from a YouTube playlist video id and raw snippet title.
func YouTubeFromRaw(videoID, rawTitle string) Track {
	title, artist := parseYouTubeTitle(rawTitle)
	return Track{
		Platform: PlatformYouTube,
		ID:       videoID,
		Title:    title,
		Artist:   artist,
		RawTitle: rawTitle,
	}
}

// SpotifyTrack builds a Track from Spotify playlist metadata.
func SpotifyTrack(id, title, artist string) Track {
	return Track{
		Platform: PlatformSpotify,
		ID:       id,
		Title:    strings.TrimSpace(title),
		Artist:   strings.TrimSpace(artist),
	}
}

// FindMatch returns the best destination candidate for source using metadata only.
func FindMatch(source Track, candidates []Track) Decision {
	decision := Decision{
		Source: source,
		Method: "normalized_metadata",
	}

	var best *Track
	bestScore := 0.0
	for i := range candidates {
		score := ScorePair(source, candidates[i])
		if score > bestScore {
			bestScore = score
			copy := candidates[i]
			best = &copy
		}
	}

	decision.Score = bestScore
	decision.BestMatch = best
	decision.Matched = bestScore >= MatchThreshold
	if decision.Matched {
		decision.SkipReason = "already_in_destination"
	}
	return decision
}

// ScorePair compares two tracks using normalized title and artist tokens.
func ScorePair(a, b Track) float64 {
	titleScore := tokenSimilarity(normalizeTitle(a.Title), normalizeTitle(b.Title))
	artistA := normalizeArtist(a.Artist)
	artistB := normalizeArtist(b.Artist)
	if artistA == "" || artistB == "" {
		// YouTube titles often embed the artist; lean on title similarity.
		return titleScore
	}
	artistScore := tokenSimilarity(artistA, artistB)
	return titleScore*0.65 + artistScore*0.35
}

// BriefAnalysisLine is a single stdout-friendly summary for analysis decisions.
func BriefAnalysisLine(d Decision, destinationService string, queued bool) string {
	src := strings.TrimSpace(d.Source.Title)
	if strings.TrimSpace(d.Source.Artist) != "" {
		src += " — " + strings.TrimSpace(d.Source.Artist)
	}
	if queued {
		best := "none"
		if d.BestMatch != nil {
			best = fmt.Sprintf("%.2f %q", d.Score, strings.TrimSpace(d.BestMatch.Title))
		}
		return fmt.Sprintf("analysis queued add %s: %q (best dest match %s)", destinationService, src, best)
	}
	if d.Matched && d.BestMatch != nil {
		return fmt.Sprintf(
			"analysis skip add %s: %q matched dest %q (score %.2f)",
			destinationService,
			src,
			strings.TrimSpace(d.BestMatch.Title),
			d.Score,
		)
	}
	return fmt.Sprintf("analysis skip add %s: %q (no dest match above threshold)", destinationService, src)
}

func normalizeTitle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	for _, re := range noisePatterns {
		s = re.ReplaceAllString(s, "")
	}
	return collapseSpaces(stripPunctuation(s))
}

func normalizeArtist(s string) string {
	return collapseSpaces(stripPunctuation(strings.ToLower(strings.TrimSpace(s))))
}

func parseYouTubeTitle(raw string) (title, artist string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	parts := strings.SplitN(raw, " - ", 2)
	if len(parts) == 2 {
		title = strings.TrimSpace(parts[0])
		rest := strings.TrimSpace(parts[1])
		if pipe := strings.SplitN(rest, " | ", 2); len(pipe) > 0 {
			artist = strings.TrimSpace(pipe[0])
		}
		artist = stripNoise(artist)
		return strings.TrimSpace(title), artist
	}
	return raw, ""
}

func tokenSimilarity(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	tokensA := strings.Fields(a)
	tokensB := strings.Fields(b)
	if len(tokensA) == 0 || len(tokensB) == 0 {
		return 0
	}
	setA := make(map[string]struct{}, len(tokensA))
	for _, t := range tokensA {
		setA[t] = struct{}{}
	}
	intersect := 0
	for _, t := range tokensB {
		if _, ok := setA[t]; ok {
			intersect++
		}
	}
	union := len(setA) + len(tokensB) - intersect
	if union == 0 {
		return 0
	}
	return float64(intersect) / float64(union)
}

func stripPunctuation(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func stripNoise(s string) string {
	s = strings.TrimSpace(s)
	for _, re := range noisePatterns {
		s = re.ReplaceAllString(s, "")
	}
	return strings.TrimSpace(s)
}
