package matching

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
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
	// Bracketed qualifiers that YouTube uploads add but Spotify titles never carry.
	regexp.MustCompile(`(?i)[\(\[][^\)\]]*\b(official|oficial|video|videoclip|audio|lyrics?|letra|visuali[sz]er|hd|4k|hq|mv|m/v)\b[^\)\]]*[\)\]]`),
	// Featured-artist credits are formatted differently on each platform; compare without them.
	regexp.MustCompile(`(?i)[\(\[]\s*(feat|ft|featuring|with)\.?\s[^\)\]]*[\)\]]`),
	regexp.MustCompile(`(?i)\s(feat|ft|featuring)\.?\s.*$`),
	// "Song x Artist2, Artist3" — collaborator lists appended to the title.
	regexp.MustCompile(`(?i)\s+x\s+.*$`),
}

var channelSuffixes = regexp.MustCompile(`(?i)(\s*-\s*topic|vevo|\s*official(\s+(channel|music))?)$`)

// YouTubeFromRaw builds a Track from a YouTube video id and raw title when the channel is unknown.
func YouTubeFromRaw(videoID, rawTitle string) Track {
	return YouTubeTrack(videoID, rawTitle, "")
}

// YouTubeTrack builds a Track from a YouTube video id, raw title, and uploader channel title.
// YouTube Music uploads use "Artist - Topic" channels with a bare song title; regular videos
// usually follow "Artist - Title".
func YouTubeTrack(videoID, rawTitle, channelTitle string) Track {
	title, artist := parseYouTubeTitle(rawTitle, cleanChannelArtist(channelTitle))
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

// ScorePair compares two tracks using normalized title and artist tokens. It also scores the
// combined title+artist token sets so a swapped or missing artist/title split still matches.
func ScorePair(a, b Track) float64 {
	titleScore := tokenSimilarity(normalizeTitle(a.Title), normalizeTitle(b.Title))
	combined := tokenSimilarity(
		normalizeTitle(a.Title+" "+a.Artist),
		normalizeTitle(b.Title+" "+b.Artist),
	)
	artistA := normalizeArtist(a.Artist)
	artistB := normalizeArtist(b.Artist)
	structured := titleScore
	if artistA != "" && artistB != "" {
		structured = titleScore*0.65 + artistSimilarity(artistA, artistB)*0.35
	}
	if combined > structured {
		return combined
	}
	return structured
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

func cleanChannelArtist(channel string) string {
	return strings.TrimSpace(channelSuffixes.ReplaceAllString(strings.TrimSpace(channel), ""))
}

// parseYouTubeTitle splits "Artist - Title" (or "Title - Artist" when the channel says so).
func parseYouTubeTitle(raw, channelArtist string) (title, artist string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", channelArtist
	}
	parts := strings.SplitN(raw, " - ", 2)
	if len(parts) != 2 {
		return stripNoise(raw), channelArtist
	}
	left := stripNoise(parts[0])
	right := strings.TrimSpace(parts[1])
	if pipe := strings.SplitN(right, " | ", 2); len(pipe) > 0 {
		right = pipe[0]
	}
	right = stripNoise(right)

	if channelArtist != "" {
		channel := normalizeArtist(channelArtist)
		if tokenOverlap(channel, normalizeArtist(right)) > tokenOverlap(channel, normalizeArtist(left)) {
			return left, right
		}
	}
	return right, left
}

// artistSimilarity tolerates multi-artist credits ("A x B" vs "A") by scoring the shorter
// credit's containment in the longer one.
func artistSimilarity(a, b string) float64 {
	if len(strings.Fields(a)) > len(strings.Fields(b)) {
		a, b = b, a
	}
	if overlap := tokenOverlap(a, b); overlap > tokenSimilarity(a, b) {
		return overlap
	}
	return tokenSimilarity(a, b)
}

// tokenOverlap is the share of a's tokens present in b.
func tokenOverlap(a, b string) float64 {
	tokensA := strings.Fields(a)
	if len(tokensA) == 0 {
		return 0
	}
	setB := make(map[string]struct{})
	for _, t := range strings.Fields(b) {
		setB[t] = struct{}{}
	}
	hits := 0
	for _, t := range tokensA {
		if _, ok := setB[t]; ok {
			hits++
		}
	}
	return float64(hits) / float64(len(tokensA))
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

// foldAccents maps "García" to "Garcia" so accented and plain spellings compare equal.
func foldAccents(s string) string {
	folded, _, err := transform.String(transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC), s)
	if err != nil {
		return s
	}
	return folded
}

func stripPunctuation(s string) string {
	s = foldAccents(s)
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
