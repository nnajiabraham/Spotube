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
	// Channel is the YouTube uploader ("Artist - Topic" for YouTube Music uploads).
	Channel string `json:"channel,omitempty"`
	// DurationMs is 0 when unknown.
	DurationMs int `json:"duration_ms,omitempty"`
}

// Decision captures whether a source track exists in a destination playlist.
type Decision struct {
	Source     Track   `json:"source"`
	BestMatch  *Track  `json:"best_match,omitempty"`
	Score      float64 `json:"score"`
	Breakdown  *Score  `json:"breakdown,omitempty"`
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
		Channel:  strings.TrimSpace(channelTitle),
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
	var bestScore *Score
	for i := range candidates {
		score := ScoreCandidate(source, candidates[i])
		if bestScore == nil || score.Total > bestScore.Total {
			bestScore = &score
			copy := candidates[i]
			best = &copy
		}
	}

	if bestScore != nil && bestScore.Total > 0 {
		decision.Score = bestScore.Total
		decision.Breakdown = bestScore
		decision.BestMatch = best
	}
	decision.Matched = decision.Score >= MatchThreshold
	if decision.Matched {
		decision.SkipReason = "already_in_destination"
	}
	return decision
}

// ScorePair is ScoreCandidate(a, b).Total.
func ScorePair(a, b Track) float64 {
	return ScoreCandidate(a, b).Total
}

// Score explains how two tracks compared. Total is in [0, 1]; MatchThreshold or more means
// "same recording".
type Score struct {
	Total  float64 `json:"total"`
	Title  float64 `json:"title"`
	Artist float64 `json:"artist"`
	// ArtistKnown is false when either side has no artist, in which case Artist is unused.
	ArtistKnown bool `json:"artist_known"`
	// DurationDiffSec is set when both durations are known.
	DurationDiffSec *int         `json:"duration_diff_sec,omitempty"`
	Adjustments     []Adjustment `json:"adjustments,omitempty"`
}

// Adjustment is a bonus or penalty applied on top of the title/artist score.
type Adjustment struct {
	Reason string  `json:"reason"`
	Delta  float64 `json:"delta"`
}

// versionTags mark recordings that differ from the studio version even when the title matches.
var versionTags = []struct {
	label string
	re    *regexp.Regexp
}{
	{"live", regexp.MustCompile(`(?i)\blive\b`)},
	{"remix", regexp.MustCompile(`(?i)\b(remix|rmx)\b`)},
	{"cover", regexp.MustCompile(`(?i)\bcover\b`)},
	{"acoustic", regexp.MustCompile(`(?i)\bacoustic\b`)},
	{"instrumental", regexp.MustCompile(`(?i)\binstrumental\b`)},
	{"karaoke", regexp.MustCompile(`(?i)\bkaraoke\b`)},
	{"sped up", regexp.MustCompile(`(?i)\bsped\s*up\b`)},
	{"slowed", regexp.MustCompile(`(?i)\bslowed\b`)},
	{"nightcore", regexp.MustCompile(`(?i)\bnightcore\b`)},
	{"8D", regexp.MustCompile(`(?i)\b8d\b`)},
	{"extended", regexp.MustCompile(`(?i)\bextended\b`)},
	{"loop", regexp.MustCompile(`(?i)\b(loop|1\s*hour|one hour)\b`)},
}

// ScoreCandidate compares two tracks on normalized title and artist, then adjusts for
// duration, version tags (live, remix, …), and official YouTube uploads.
func ScoreCandidate(a, b Track) Score {
	var s Score
	s.Title = textSimilarity(normalizeTitle(a.Title), normalizeTitle(b.Title))
	base := s.Title

	artistA, artistB := normalizeArtist(a.Artist), normalizeArtist(b.Artist)
	if artistA != "" && artistB != "" {
		s.ArtistKnown = true
		s.Artist = artistSimilarity(artistA, artistB)
		// The uploader channel is often the artist even when the title was parsed differently.
		if ch := normalizeArtist(cleanChannelArtist(b.Channel)); ch != "" {
			s.Artist = max(s.Artist, artistSimilarity(artistA, ch))
		}
		if ch := normalizeArtist(cleanChannelArtist(a.Channel)); ch != "" {
			s.Artist = max(s.Artist, artistSimilarity(ch, artistB))
		}
		base = s.Title*0.6 + s.Artist*0.4
	}
	// Title and artist swapped or merged ("Artist - Title" parsed the wrong way round).
	combined := textSimilarity(normalizeTitle(a.Title+" "+a.Artist), normalizeTitle(b.Title+" "+b.Artist))
	if combined > base {
		base = combined
	}

	adjust := func(reason string, delta float64) {
		s.Adjustments = append(s.Adjustments, Adjustment{Reason: reason, Delta: delta})
		base += delta
	}

	if a.DurationMs > 0 && b.DurationMs > 0 {
		diff := a.DurationMs - b.DurationMs
		if diff < 0 {
			diff = -diff
		}
		sec := diff / 1000
		s.DurationDiffSec = &sec
		switch {
		case sec <= 3:
			adjust("same length", 0.05)
		case sec <= 10:
			adjust("length within 10s", 0.02)
		case sec <= 45:
			// Music videos often add an intro or outro; no signal either way.
		case sec <= 120:
			adjust(fmt.Sprintf("length differs by %ds", sec), -0.08)
		case sec <= 300:
			adjust(fmt.Sprintf("length differs by %s", minutes(sec)), -0.2)
		default:
			adjust(fmt.Sprintf("length differs by %s", minutes(sec)), -0.4)
		}
	}

	textA, textB := versionText(a), versionText(b)
	for _, tag := range versionTags {
		inA, inB := tag.re.MatchString(textA), tag.re.MatchString(textB)
		if inA != inB {
			adjust(fmt.Sprintf("only one is %s", tag.label), -0.2)
		}
	}

	for _, t := range []Track{a, b} {
		if t.Platform != PlatformYouTube || t.Channel == "" {
			continue
		}
		if strings.HasSuffix(strings.ToLower(strings.TrimSpace(t.Channel)), "- topic") {
			adjust("YouTube Music upload", 0.03)
		}
	}

	s.Total = clamp01(base)
	return s
}

func versionText(t Track) string {
	if t.RawTitle != "" {
		return t.RawTitle
	}
	return t.Title
}

func minutes(sec int) string {
	return fmt.Sprintf("%dm%02ds", sec/60, sec%60)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return float64(int(v*1000+0.5)) / 1000
}

// textSimilarity is the larger of token Jaccard and character-trigram Dice, so word-order
// changes and small spelling differences both score well.
func textSimilarity(a, b string) float64 {
	tokens := tokenSimilarity(a, b)
	if tri := trigramSimilarity(a, b); tri > tokens {
		return tri
	}
	return tokens
}

func trigramSimilarity(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	ga, gb := trigrams(a), trigrams(b)
	if len(ga) == 0 || len(gb) == 0 {
		return 0
	}
	shared := 0
	for g, n := range ga {
		if m, ok := gb[g]; ok {
			shared += min(n, m)
		}
	}
	total := 0
	for _, n := range ga {
		total += n
	}
	for _, n := range gb {
		total += n
	}
	return 2 * float64(shared) / float64(total)
}

func trigrams(s string) map[string]int {
	r := []rune("  " + s + " ")
	out := make(map[string]int, len(r))
	for i := 0; i+3 <= len(r); i++ {
		out[string(r[i:i+3])]++
	}
	return out
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
