package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/zmb3/spotify"
)

// redirectURI is the OAuth redirect URI for the application.
// You must register an application at Spotify's developer portal
// and enter this value.

var (
	clientID= "0c411be60c2943679ce623ca8055126d"
	clientSecret = "4d8bbadad5bc41dab9490d2e304f7881"
	scopes= "user-read-private user-read-email playlist-read-private playlist-read-collaborative"
	redirectURICallback= "https://741604de.ngrok.io/spotify-callback"
	ch    = make(chan *spotify.Client)
	state = "abc123"
	auth = spotify.NewAuthenticator(redirectURICallback, scopes)
	client = <-ch
)

func getSpotifyAuthLoginURL() string{
	auth.SetAuthInfo(clientID, clientSecret)
	url := auth.AuthURL("state1")
	return url
}

func mains() {
	// first start an HTTP server

	// url := auth.AuthURL(state)
	// fmt.Println("Please log in to Spotify by visiting the following page in your browser:", url)

	// wait for auth to complete
	
	// client := <-ch
	// use the client to make calls that require authorization
	// user, err := client.CurrentUser()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Println("You are logged in as:", user.ID)
}

func spotifyCallback(w http.ResponseWriter, r *http.Request) {
	token, err := auth.Token(state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Fatal(err)
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Fatalf("State mismatch: %s != %s\n", st, state)
	}
	// use the token to get an authenticated client
	client := auth.NewClient(token)
	fmt.Fprintf(w, "Login Completed!")
	ch <- &client
}