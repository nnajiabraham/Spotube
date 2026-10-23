// Package platform wraps the Spotify and YouTube APIs used by the sync engine and the
// in-app track search.
//
// YouTube Data API quota costs (10,000 units/day by default): search 100, insert/update 50,
// any list call 1 (videos.list takes up to 50 IDs per call).
package platform

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"

	spotify "github.com/zmb3/spotify/v2"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/youtube/v3"

	"github.com/manlikeabro/spotube/internal/auth"
	"github.com/manlikeabro/spotube/internal/matching"
)

// youtubeMusicCategory is the "Music" video category; searching inside it drops sermons,
// vlogs and reaction videos that share a song's title.
const youtubeMusicCategory = "10"

// SearchHit is a track or video returned by a search or lookup.
type SearchHit struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist,omitempty"`
	// Channel is the YouTube uploader.
	Channel string `json:"channel,omitempty"`
	// Album is the Spotify album name.
	Album string `json:"album,omitempty"`
	// RawTitle is the YouTube video title as uploaded.
	RawTitle   string `json:"raw_title,omitempty"`
	DurationMs int    `json:"duration_ms,omitempty"`
	Image      string `json:"image,omitempty"`
}

// Track converts a hit into a matching.Track for scoring.
func (h SearchHit) Track(p matching.Platform) matching.Track {
	return matching.Track{
		Platform:   p,
		ID:         h.ID,
		Title:      h.Title,
		Artist:     h.Artist,
		RawTitle:   h.RawTitle,
		Channel:    h.Channel,
		DurationMs: h.DurationMs,
	}
}

// Spotify calls the Spotify Web API with the stored OAuth token.
type Spotify struct {
	Factory *auth.ClientFactory
}

// SearchTracks finds tracks for a known title/artist, trying a field-filtered query first.
func (p *Spotify) SearchTracks(ctx context.Context, title, artist string, limit int) ([]SearchHit, error) {
	title, artist = strings.TrimSpace(title), strings.TrimSpace(artist)
	if title == "" && artist == "" {
		return nil, nil
	}
	if title != "" && artist != "" {
		hits, err := p.Search(ctx, fmt.Sprintf("track:%q artist:%q", title, artist), limit)
		if err != nil || len(hits) > 0 {
			return hits, err
		}
	}
	return p.Search(ctx, strings.TrimSpace(title+" "+artist), limit)
}

// Search runs a free-text track search.
func (p *Spotify) Search(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	client, err := p.Factory.GetSpotifyClient(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 5
	}
	result, err := client.Search(ctx, query, spotify.SearchTypeTrack, spotify.Limit(limit))
	if err != nil {
		return nil, err
	}
	if result.Tracks == nil {
		return nil, nil
	}
	hits := make([]SearchHit, 0, len(result.Tracks.Tracks))
	for _, track := range result.Tracks.Tracks {
		hits = append(hits, spotifyHit(track))
	}
	return hits, nil
}

// GetTrack looks up one track by ID.
func (p *Spotify) GetTrack(ctx context.Context, id string) (*SearchHit, error) {
	client, err := p.Factory.GetSpotifyClient(ctx)
	if err != nil {
		return nil, err
	}
	track, err := client.GetTrack(ctx, spotify.ID(id))
	if err != nil {
		return nil, err
	}
	hit := spotifyHit(*track)
	return &hit, nil
}

// ListPlaylistTracks returns every track in a playlist.
func (p *Spotify) ListPlaylistTracks(ctx context.Context, playlistID string) ([]matching.Track, error) {
	client, err := p.Factory.GetSpotifyClient(ctx)
	if err != nil {
		return nil, err
	}
	return SpotifyPlaylistTracks(ctx, client, playlistID)
}

func (p *Spotify) AddTrackToPlaylist(ctx context.Context, playlistID, trackID string) error {
	client, err := p.Factory.GetSpotifyClient(ctx)
	if err != nil {
		return err
	}
	_, err = client.AddTracksToPlaylist(ctx, spotify.ID(playlistID), spotify.ID(trackID))
	return err
}

func (p *Spotify) RenamePlaylist(ctx context.Context, playlistID, name string) error {
	client, err := p.Factory.GetSpotifyClient(ctx)
	if err != nil {
		return err
	}
	return client.ChangePlaylistName(ctx, spotify.ID(playlistID), name)
}

// SpotifyPlaylistTracks pages through a playlist (100 tracks per request).
func SpotifyPlaylistTracks(ctx context.Context, client *spotify.Client, playlistID string) ([]matching.Track, error) {
	var tracks []matching.Track
	offset := 0
	for {
		page, err := client.GetPlaylistItems(ctx, spotify.ID(playlistID), spotify.Offset(offset), spotify.Limit(100))
		if err != nil {
			return nil, err
		}
		for _, item := range page.Items {
			if item.Track.Track == nil {
				continue
			}
			track := item.Track.Track
			t := matching.SpotifyTrack(string(track.ID), track.Name, firstArtist(track.Artists))
			t.DurationMs = int(track.Duration)
			if images := track.Album.Images; len(images) > 0 {
				t.Image = images[len(images)-1].URL
			}
			tracks = append(tracks, t)
		}
		if page.Next == "" || len(page.Items) == 0 {
			break
		}
		offset += len(page.Items)
	}
	return tracks, nil
}

func spotifyHit(track spotify.FullTrack) SearchHit {
	hit := SearchHit{
		ID:         string(track.ID),
		Title:      track.Name,
		Artist:     joinArtists(track.Artists),
		Album:      track.Album.Name,
		DurationMs: int(track.Duration),
	}
	if images := track.Album.Images; len(images) > 0 {
		// Images are largest first; the last one is the thumbnail.
		hit.Image = images[len(images)-1].URL
	}
	return hit
}

func firstArtist(artists []spotify.SimpleArtist) string {
	if len(artists) == 0 {
		return ""
	}
	return artists[0].Name
}

func joinArtists(artists []spotify.SimpleArtist) string {
	names := make([]string, 0, len(artists))
	for _, a := range artists {
		names = append(names, a.Name)
	}
	return strings.Join(names, ", ")
}

// YouTube calls the YouTube Data API v3 with the stored Google OAuth token.
type YouTube struct {
	Factory *auth.ClientFactory
}

// SearchVideos finds music videos for a known title/artist (101 quota units).
func (p *YouTube) SearchVideos(ctx context.Context, title, artist string, limit int) ([]SearchHit, error) {
	query := strings.TrimSpace(artist + " " + title)
	if query == "" {
		return nil, nil
	}
	return p.Search(ctx, query, limit)
}

// Search runs a free-text search in the Music category, then fetches durations
// (100 + 1 quota units).
func (p *YouTube) Search(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	service, err := p.Factory.GetYouTubeService(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 5
	}
	response, err := service.Search.List([]string{"snippet"}).
		Q(query).
		Type("video").
		VideoCategoryId(youtubeMusicCategory).
		MaxResults(int64(limit)).
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(response.Items))
	for _, item := range response.Items {
		if item.Id != nil && item.Id.VideoId != "" {
			ids = append(ids, item.Id.VideoId)
		}
	}
	return YouTubeVideos(ctx, service, ids)
}

// GetVideo looks up one video by ID (1 quota unit).
func (p *YouTube) GetVideo(ctx context.Context, id string) (*SearchHit, error) {
	service, err := p.Factory.GetYouTubeService(ctx)
	if err != nil {
		return nil, err
	}
	hits, err := YouTubeVideos(ctx, service, []string{id})
	if err != nil {
		return nil, err
	}
	if len(hits) == 0 {
		return nil, nil
	}
	return &hits[0], nil
}

func (p *YouTube) ListPlaylistTracks(ctx context.Context, playlistID string) ([]matching.Track, error) {
	service, err := p.Factory.GetYouTubeService(ctx)
	if err != nil {
		return nil, err
	}
	return YouTubePlaylistTracks(ctx, service, playlistID)
}

func (p *YouTube) AddVideoToPlaylist(ctx context.Context, playlistID, videoID string) error {
	service, err := p.Factory.GetYouTubeService(ctx)
	if err != nil {
		return err
	}
	item := &youtube.PlaylistItem{
		Snippet: &youtube.PlaylistItemSnippet{
			PlaylistId: playlistID,
			ResourceId: &youtube.ResourceId{Kind: "youtube#video", VideoId: videoID},
		},
	}
	_, err = service.PlaylistItems.Insert([]string{"snippet"}, item).Context(ctx).Do()
	return err
}

func (p *YouTube) RenamePlaylist(ctx context.Context, playlistID, title string) error {
	service, err := p.Factory.GetYouTubeService(ctx)
	if err != nil {
		return err
	}
	playlist := &youtube.Playlist{Id: playlistID, Snippet: &youtube.PlaylistSnippet{Title: title}}
	_, err = service.Playlists.Update([]string{"snippet"}, playlist).Context(ctx).Do()
	return err
}

// YouTubePlaylistTracks lists a playlist (1 unit per 50 videos) and fills in durations
// (another 1 unit per 50 videos).
func YouTubePlaylistTracks(ctx context.Context, service *youtube.Service, playlistID string) ([]matching.Track, error) {
	return YouTubePlaylistTracksFor(ctx, service, playlistID, nil)
}

// YouTubePlaylistTracksFor lists a playlist and fetches durations only for videos where
// needDuration returns true (all when nil). Durations only help match unknown videos, so a
// sync skips them for videos it has already linked.
func YouTubePlaylistTracksFor(ctx context.Context, service *youtube.Service, playlistID string, needDuration func(videoID string) bool) ([]matching.Track, error) {
	var tracks []matching.Track
	pageToken := ""
	for {
		call := service.PlaylistItems.List([]string{"snippet", "contentDetails"}).
			PlaylistId(playlistID).
			MaxResults(50).
			Context(ctx)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		response, err := call.Do()
		if err != nil {
			if isNotFound(err) {
				return nil, ErrPlaylistNotFound
			}
			return nil, err
		}
		ids := make([]string, 0, len(response.Items))
		for _, item := range response.Items {
			if item.ContentDetails == nil || item.ContentDetails.VideoId == "" || item.Snippet == nil {
				continue
			}
			id := item.ContentDetails.VideoId
			if needDuration == nil || needDuration(id) {
				ids = append(ids, id)
			}
			tracks = append(tracks, matching.YouTubeTrack(
				id,
				html.UnescapeString(item.Snippet.Title),
				item.Snippet.VideoOwnerChannelTitle,
			))
		}
		if len(ids) > 0 {
			durations, err := youtubeDurations(ctx, service, ids)
			if err != nil {
				return nil, err
			}
			for i := range tracks {
				if d, ok := durations[tracks[i].ID]; ok && tracks[i].DurationMs == 0 {
					tracks[i].DurationMs = d
				}
			}
		}
		pageToken = response.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return tracks, nil
}

// ErrPlaylistNotFound means the playlist was deleted or is no longer visible to the account.
var ErrPlaylistNotFound = errors.New("playlist not found (deleted, or no longer visible to this account)")

// YouTubePlaylistInfo returns a playlist's title and itemCount (1 quota unit). itemCount
// includes private and deleted videos and can lag a few seconds behind inserts.
func YouTubePlaylistInfo(ctx context.Context, service *youtube.Service, playlistID string) (title string, itemCount int, err error) {
	response, err := service.Playlists.List([]string{"snippet", "contentDetails"}).Id(playlistID).Context(ctx).Do()
	if err != nil {
		if isNotFound(err) {
			return "", 0, ErrPlaylistNotFound
		}
		return "", 0, err
	}
	if len(response.Items) == 0 {
		return "", 0, ErrPlaylistNotFound
	}
	p := response.Items[0]
	if p.Snippet != nil {
		title = strings.TrimSpace(p.Snippet.Title)
	}
	if p.ContentDetails != nil {
		itemCount = int(p.ContentDetails.ItemCount)
	}
	return title, itemCount, nil
}

// SpotifyPlaylistName returns a playlist's name (free).
func SpotifyPlaylistName(ctx context.Context, client *spotify.Client, playlistID string) (string, error) {
	playlist, err := client.GetPlaylist(ctx, spotify.ID(playlistID), spotify.Fields("name"))
	if err != nil {
		if isNotFound(err) {
			return "", ErrPlaylistNotFound
		}
		return "", err
	}
	return strings.TrimSpace(playlist.Name), nil
}

func isNotFound(err error) bool {
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		return gerr.Code == http.StatusNotFound
	}
	var serr spotify.Error
	if errors.As(err, &serr) {
		return serr.Status == http.StatusNotFound
	}
	return false
}

// YouTubeVideos fetches title, channel, duration and thumbnail for up to 50 videos
// (1 quota unit), keeping the order of ids.
func YouTubeVideos(ctx context.Context, service *youtube.Service, ids []string) ([]SearchHit, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	response, err := service.Videos.List([]string{"snippet", "contentDetails"}).
		Id(ids...).
		MaxResults(int64(len(ids))).
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*youtube.Video, len(response.Items))
	for _, v := range response.Items {
		byID[v.Id] = v
	}
	hits := make([]SearchHit, 0, len(ids))
	for _, id := range ids {
		v, ok := byID[id]
		if !ok || v.Snippet == nil {
			continue
		}
		raw := html.UnescapeString(v.Snippet.Title)
		parsed := matching.YouTubeTrack(id, raw, v.Snippet.ChannelTitle)
		hit := SearchHit{
			ID:       id,
			Title:    parsed.Title,
			Artist:   parsed.Artist,
			Channel:  v.Snippet.ChannelTitle,
			RawTitle: raw,
		}
		if v.ContentDetails != nil {
			hit.DurationMs = ParseISODuration(v.ContentDetails.Duration)
		}
		if t := v.Snippet.Thumbnails; t != nil {
			switch {
			case t.Medium != nil:
				hit.Image = t.Medium.Url
			case t.Default != nil:
				hit.Image = t.Default.Url
			}
		}
		hits = append(hits, hit)
	}
	return hits, nil
}

func youtubeDurations(ctx context.Context, service *youtube.Service, ids []string) (map[string]int, error) {
	out := make(map[string]int, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	response, err := service.Videos.List([]string{"contentDetails"}).Id(ids...).MaxResults(50).Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	for _, v := range response.Items {
		if v.ContentDetails != nil {
			out[v.Id] = ParseISODuration(v.ContentDetails.Duration)
		}
	}
	return out, nil
}

// ParseISODuration converts YouTube's ISO 8601 durations ("PT1H2M3S") to milliseconds.
func ParseISODuration(s string) int {
	s = strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(s)), "P")
	total, n := 0, 0
	inTime := false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			n = n*10 + int(r-'0')
		case r == 'T':
			inTime = true
		case r == 'D':
			total += n * 86400
			n = 0
		case r == 'H' && inTime:
			total += n * 3600
			n = 0
		case r == 'M' && inTime:
			total += n * 60
			n = 0
		case r == 'S' && inTime:
			total += n
			n = 0
		default:
			n = 0
		}
	}
	return total * 1000
}
