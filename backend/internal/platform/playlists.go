package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	spotify "github.com/zmb3/spotify/v2"
	"google.golang.org/api/youtube/v3"

	"github.com/manlikeabro/spotube/internal/auth"
	"github.com/manlikeabro/spotube/internal/matching"
)

// spotifyAPI is the Spotify Web API base URL; tests point it at a fake server.
var spotifyAPI = "https://api.spotify.com/v1"

// Playlists reads and writes whole playlists on either service, for backups and restores.
type Playlists struct {
	Factory *auth.ClientFactory
}

// PlaylistTracks returns a playlist's name and tracks in order. Spotify is free; YouTube costs
// 1 unit for the name plus 2 units per 50 videos.
func (p *Playlists) PlaylistTracks(ctx context.Context, service, playlistID string) (string, []matching.Track, error) {
	switch service {
	case "spotify":
		client, err := p.Factory.GetSpotifyClient(ctx)
		if err != nil {
			return "", nil, err
		}
		playlist, err := client.GetPlaylist(ctx, spotify.ID(playlistID), spotify.Fields("name"))
		if err != nil {
			return "", nil, err
		}
		tracks, err := SpotifyPlaylistTracks(ctx, client, playlistID)
		return playlist.Name, tracks, err
	case "youtube":
		service, err := p.Factory.GetYouTubeService(ctx)
		if err != nil {
			return "", nil, err
		}
		res, err := service.Playlists.List([]string{"snippet"}).Id(playlistID).Context(ctx).Do()
		if err != nil {
			return "", nil, err
		}
		if len(res.Items) == 0 || res.Items[0].Snippet == nil {
			return "", nil, fmt.Errorf("YouTube playlist %s not found", playlistID)
		}
		tracks, err := YouTubePlaylistTracks(ctx, service, playlistID)
		return res.Items[0].Snippet.Title, tracks, err
	}
	return "", nil, errors.New("unknown service: " + service)
}

// CreatePlaylist creates a playlist (private on YouTube; see below for Spotify) and returns its ID
// (YouTube: 50 units).
func (p *Playlists) CreatePlaylist(ctx context.Context, service, name, description string) (string, error) {
	switch service {
	case "spotify":
		// POST /me/playlists: the per-user endpoint is not available to apps in development mode.
		// Spotify currently ignores "public": false (also via a later PUT), so new playlists show as public.
		client, err := p.Factory.HTTPClient(ctx, "spotify")
		if err != nil {
			return "", err
		}
		body, _ := json.Marshal(map[string]any{"name": name, "description": description, "public": false})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, spotifyAPI+"/me/playlists", bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		res, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusOK {
			return "", fmt.Errorf("spotify: create playlist failed (%d): %s", res.StatusCode, bytes.TrimSpace(raw))
		}
		var created struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &created); err != nil || created.ID == "" {
			return "", fmt.Errorf("spotify: unexpected create playlist response: %s", bytes.TrimSpace(raw))
		}
		return created.ID, nil
	case "youtube":
		service, err := p.Factory.GetYouTubeService(ctx)
		if err != nil {
			return "", err
		}
		playlist := &youtube.Playlist{
			Snippet: &youtube.PlaylistSnippet{Title: name, Description: description},
			Status:  &youtube.PlaylistStatus{PrivacyStatus: "private"},
		}
		created, err := service.Playlists.Insert([]string{"snippet", "status"}, playlist).Context(ctx).Do()
		if err != nil {
			return "", err
		}
		return created.Id, nil
	}
	return "", errors.New("unknown service: " + service)
}

// AddTracks appends ids to a playlist in order: Spotify in batches of 100 (free), YouTube one
// video at a time (50 units each).
func (p *Playlists) AddTracks(ctx context.Context, service, playlistID string, ids []string) (int, error) {
	added := 0
	switch service {
	case "spotify":
		client, err := p.Factory.GetSpotifyClient(ctx)
		if err != nil {
			return 0, err
		}
		for start := 0; start < len(ids); start += 100 {
			end := min(start+100, len(ids))
			batch := make([]spotify.ID, 0, end-start)
			for _, id := range ids[start:end] {
				batch = append(batch, spotify.ID(id))
			}
			if _, err := client.AddTracksToPlaylist(ctx, spotify.ID(playlistID), batch...); err != nil {
				return added, err
			}
			added += len(batch)
		}
		return added, nil
	case "youtube":
		yt := &YouTube{Factory: p.Factory}
		for _, id := range ids {
			if err := yt.AddVideoToPlaylist(ctx, playlistID, id); err != nil {
				return added, err
			}
			added++
		}
		return added, nil
	}
	return 0, errors.New("unknown service: " + service)
}
