package services

import (
	"errors"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/nnajiabraham/spotube/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/youtube/v3"
)

type YoutubeService struct{
	Config *config.Configs
	googleClientSecretFile []byte
	googleOauthConfig *oauth2.Config
}

// var oauthConfig = &oauth2.Config{
//         ClientID:     "", // from https://console.developers.google.com/project/<your-project-id>/apiui/credential
//         ClientSecret: "", // from https://console.developers.google.com/project/<your-project-id>/apiui/credential
//         Endpoint:     google.Endpoint,
//         Scopes:       []string{youtube.YoutubeScope},
// }

func (s *YoutubeService) getGoogleClientSecretFile() []byte{
	log.Printf("fetching googleClientSecretFile")

	if s.googleClientSecretFile != nil {
		return s.googleClientSecretFile
	}

	googleClientSecretFile, err := ioutil.ReadFile("client_secret.json")
	if err != nil {
	log.Fatalf("Unable to read client secret file: %v", err)
	}

	log.Printf("googleClientSecretFile %s", googleClientSecretFile)

	return googleClientSecretFile
}

func (s *YoutubeService) getGoogleConfigAuth() *oauth2.Config{
	log.Printf("getting config ")

	if s.googleOauthConfig != nil {
		return s.googleOauthConfig
	}

	googleConfig, err := google.ConfigFromJSON(s.getGoogleClientSecretFile(), youtube.YoutubeReadonlyScope, youtube.YoutubeScope)
	
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	log.Printf("googleConfig %s", googleConfig)
	
	return googleConfig
}

func (s *YoutubeService) GetYoutubeAuthLoginURL() string {
	authURL := s.getGoogleConfigAuth().AuthCodeURL(s.Config.TOKEN_STATE)
	return authURL
}

func (s *YoutubeService) GetYoutubeService(r *http.Request) (*youtube.Service, error) {
	token, err := s.token(s.Config.TOKEN_STATE, r)
	if err!=nil {
		return nil, err
	}

	client:= s.getGoogleConfigAuth().Client(r.Context(), token)
	service, err := youtube.New(client)
	if err!=nil {
		return nil, err
	}

	return service, nil
}

// Token pulls an authorization code from an HTTP request and attempts to exchange
// it for an access token.  The standard use case is to call Token from the handler
// that handles requests to your application's redirect URL.
func (s *YoutubeService) token(state string, r *http.Request) (*oauth2.Token, error) {
	values := r.URL.Query()
	if e := values.Get("error"); e != "" {
		return nil, errors.New("spotify: auth failed - " + e)
	}
	code := values.Get("code")
	if code == "" {
		return nil, errors.New("spotify: didn't get access code")
	}
	actualState := values.Get("state")
	if actualState != state {
		return nil, errors.New("spotify: redirect state parameter doesn't match")
	}
	return s.getGoogleConfigAuth().Exchange(r.Context(), code)
}
