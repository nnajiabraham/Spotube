package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	uuid "github.com/gofrs/uuid"
	"github.com/nnajiabraham/spotube/models"
	"github.com/nnajiabraham/spotube/services"
	"github.com/zmb3/spotify"
)
 
type AppHandlers struct{
	UserService *services.UserService
}

var (
	clientID				= os.Getenv("SPOTIFY_ID")
	clientSecret			= os.Getenv("SPOTIFY_SECRET")
	scopes					= "user-read-private user-read-email playlist-read-private playlist-read-collaborative"
	redirectURICallback		= "http://nnajiabraham.serverless.social/spotify-callback"

	// clientChannel    = make(chan *spotify.Client)
	state = "abc123"
	auth = spotify.NewAuthenticator(redirectURICallback, scopes)
)


func (h *AppHandlers) HomeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome home from new handler!")
}

// redirectURI is the OAuth redirect URI for the application.
// You must register an application at Spotify's developer portal
// and enter this value.

//lt --port 2580 --subdomain nnajiabraham 
// lt -h "http://serverless.social" --port 2580 --open true --subdomain nnajiabraham


func getSpotifyAuthLoginURL() string{
	auth.SetAuthInfo(clientID, clientSecret)
	url := auth.AuthURL(state)
	return url
}

func (h *AppHandlers) SpotifyLogin(w http.ResponseWriter, r *http.Request) {

	fmt.Println("getting url and redirecting")
	url:= getSpotifyAuthLoginURL()
	
	
	fmt.Println("Redirect URL %s\n", url)
	http.Redirect(w, r, url, 301)
}

func (h *AppHandlers) SpotifyCallback(w http.ResponseWriter, r *http.Request) {
	log.Println("Callback hit \n")

	token, err := auth.Token(state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Fatal(err)
	}
	// use the token to get an authenticated client
	client := auth.NewClient(token)

	log.Println("Login Completed!")

	user, _ := client.CurrentUser()
	// clientChannel <- &clients
	// // userPlaylist, err := client.CurrentUsersPlaylists()

	// if err != nil {
	// 	log.Fatal(err)
	// }
	// json.NewEncoder(w).Encode(token)
	newUUID, err := uuid.NewV4()
	if err != nil {
		fmt.Printf("Something went wrong generating UUID: %s", err)
		return
	}

	newUser := &models.User{UserId: newUUID.String(),
		 Username: user.DisplayName, 
		 Email: user.Email, 
		 SpotifyId: user.ID, 
		 SpotifyToken: token.AccessToken, 
		 SpotifyRefreshToken: token.RefreshToken,}

	h.UserService.FetchOrCreateUser(newUser)

	tokenString, err := h.UserService.CreateToken(*newUser)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return;
	}
	w.Header().Add("Auth", tokenString)
	w.Write([]byte(tokenString))
	json.NewEncoder(w).Encode(newUser)
}

func (h *AppHandlers) SpotifyPlaylist(w http.ResponseWriter, r *http.Request){
	// client := <-clientChannel
	// token, err := client.Token()
	// // userPlaylist, err := client.CurrentUsersPlaylists()

	// if err != nil {
	// 	log.Fatal(err)
	// }
	// json.NewEncoder(w).Encode(token)
	fmt.Print("sdf")
}






// func createEvent(w http.ResponseWriter, r *http.Request) {
// 	var newEvent event
// 	reqBody, err := ioutil.ReadAll(r.Body)

// 	if err != nil {
// 		fmt.Fprintf(w, "Kindly enter data with the event title and description only in order to update")
// 	}
	
// 	json.Unmarshal(reqBody, &newEvent)

// 	events = append(events, newEvent)
// 	w.WriteHeader(http.StatusCreated)

// 	json.NewEncoder(w).Encode(newEvent)
// }