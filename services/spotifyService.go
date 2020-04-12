package services

import (
	"fmt"
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

type SpotifyClientToken struct{
	SpotifyClient spotify.Client
	UserToken *oauth2.Token
}

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

func (s *SpotifyService) GetSpotifyAuthLoginURL() string{
	url := s.GetSpotifyAuth().AuthURL(s.Config.TOKEN_STATE)
	return url
}


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