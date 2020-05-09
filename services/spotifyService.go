package services

import (
	"fmt"
	"log"
	"math"
	"net/http"

	"github.com/nnajiabraham/spotube/config"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)

// SpotifyService  ....
type SpotifyService struct{
	Config *config.Configs
	spotifyAuth *spotify.Authenticator
}

//SpotifyClientToken struct wraps the spotify library for custom usage
type SpotifyClientToken struct{
	SpotifyClient spotify.Client
	UserToken *oauth2.Token
}

//GetSpotifyAuth returns a spotify auth that can be used to generate a client
func (s *SpotifyService) GetSpotifyAuth() *spotify.Authenticator{
	if s.spotifyAuth != nil {
		return s.spotifyAuth
	}

	scopes					:= fmt.Sprintf("%s %s %s %s", spotify.ScopeUserReadPrivate, spotify.ScopeUserReadEmail, spotify.ScopePlaylistReadPrivate, spotify.ScopePlaylistReadCollaborative)
	redirectURICallback		:= "http://nnajiabraham.viewshd.com/spotify-callback" 
	auth := spotify.NewAuthenticator(redirectURICallback, scopes)
	auth.SetAuthInfo(s.Config.SPOTIFY_ID, s.Config.SPOTIFY_SECRET)
	s.spotifyAuth=&auth
	return &auth
}

//GetSpotifyAuthLoginURL returns a spotify login url for the client
func (s *SpotifyService) GetSpotifyAuthLoginURL() string{
	url := s.GetSpotifyAuth().AuthURL(s.Config.TOKEN_STATE)
	return url
}

//GetSpotifyClientToken returns a spotify clientToken from URL during the code-token exchange
func (s *SpotifyService) GetSpotifyClientToken(r *http.Request)(*SpotifyClientToken, error){
	token, err := s.GetSpotifyAuth().Token(s.Config.TOKEN_STATE, r)
	if err != nil {
        return nil, err
	}

	// use the token to get an authenticated client
	client := s.GetSpotifyAuth().NewClient(token)
	clientToken := &SpotifyClientToken{SpotifyClient: client, UserToken:token}
	return clientToken, nil
}

//GetUserPlaylists paginates and returns a slice of all playlists for authenticated user
func (s *SpotifyService) GetUserPlaylists(userOauthToken *oauth2.Token)([]spotify.SimplePlaylist, error){

	client:= s.GetSpotifyAuth().NewClient(userOauthToken)

	offset, limit := 0, 20
	
	options := &spotify.Options{
		Offset: &offset, 
		Limit: &limit,
	}

	userPlaylist := []spotify.SimplePlaylist{}

	initialPlaylist, err := client.CurrentUsersPlaylistsOpt(options)
	if err != nil{
		return nil, err
	}

	for _, playlist := range initialPlaylist.Playlists{
		userPlaylist = append(userPlaylist, playlist)
	}

	if initialPlaylist.Total <= 20 {
		return userPlaylist, nil
	}

	noOfPlaylistPages:= int(math.Ceil(float64(initialPlaylist.Total) / float64(limit)))

	for page:=1; page<noOfPlaylistPages;{
		page++
		nextOffset := (limit*page)-limit
		options.Offset = &nextOffset
		nextPlaylists, err := client.CurrentUsersPlaylistsOpt(options)

		if err != nil{
			log.Printf("Unable to get users playlist: %s ",err.Error())
			return nil, err
		}

		for _, playlist := range nextPlaylists.Playlists{
			userPlaylist = append(userPlaylist, playlist)
		}
	}

	return userPlaylist, nil
}