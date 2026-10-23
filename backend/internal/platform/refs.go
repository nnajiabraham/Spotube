package platform

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	spotifyIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{22}$`)
	youtubeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
)

// ParseSpotifyTrackRef accepts a track link (open.spotify.com/track/…), a URI
// (spotify:track:…) or a bare ID.
func ParseSpotifyTrackRef(input string) (string, bool) {
	input = strings.TrimSpace(input)
	if rest, ok := strings.CutPrefix(input, "spotify:track:"); ok {
		input = rest
	} else if u, err := url.Parse(input); err == nil && strings.HasSuffix(u.Host, "spotify.com") {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		for i := 0; i+1 < len(parts); i++ {
			if parts[i] == "track" {
				input = parts[i+1]
				break
			}
		}
	}
	if spotifyIDPattern.MatchString(input) {
		return input, true
	}
	return "", false
}

// ParseYouTubeVideoRef accepts youtube.com/watch?v=…, music.youtube.com, youtu.be/…,
// /shorts/… links or a bare video ID.
func ParseYouTubeVideoRef(input string) (string, bool) {
	input = strings.TrimSpace(input)
	if u, err := url.Parse(input); err == nil && u.Host != "" {
		host := strings.TrimPrefix(u.Host, "www.")
		switch {
		case host == "youtu.be":
			input = strings.Trim(u.Path, "/")
		case strings.HasSuffix(host, "youtube.com"):
			if v := u.Query().Get("v"); v != "" {
				input = v
			} else if rest, ok := strings.CutPrefix(u.Path, "/shorts/"); ok {
				input = strings.Trim(rest, "/")
			}
		}
	}
	if youtubeIDPattern.MatchString(input) {
		return input, true
	}
	return "", false
}
