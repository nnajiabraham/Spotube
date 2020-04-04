package routes

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/nnajiabraham/spotube/services"
	"github.com/zmb3/spotify"
)
 
type AppHandler struct{
	UserService *services.UserService
	TokenService *services.TokenService
}

type UserDto struct{
	UserId string
	UserName string
}

var (
	scopes					= "user-read-private user-read-email playlist-read-private playlist-read-collaborative"
	redirectURICallback		= "http://nnajiabraham.serverless.social/spotify-callback"

	// clientChannel    = make(chan *spotify.Client)
	state = "abc123"
	auth = spotify.NewAuthenticator(redirectURICallback, scopes)
)

// RegisterRoutes registers all routes paths with handlers.
func (h *AppHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/", h.HomeHandler)
	router.HandleFunc("/spotify-login", h.SpotifyLogin)
	router.HandleFunc("/spotify-callback", h.SpotifyCallback)
	router.HandleFunc("/spotify-playlist", h.SpotifyPlaylist).Methods("GET")
	router.HandleFunc("user", h.getUser).Methods("GET")
}

//npm install -g localtunnel
//npx localtunnel --port 8000
//lt --port 2580 --subdomain nnajiabraham 
// lt -h "http://serverless.social" --port 2580 --open true --subdomain nnajiabraham


func (h *AppHandler) HomeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome home from new handler!")
}

func getSpotifyAuthLoginURL() string{
	clientID := os.Getenv("SPOTIFY_ID")
	clientSecret := os.Getenv("SPOTIFY_SECRET") 
	auth.SetAuthInfo(clientID, clientSecret)
	url := auth.AuthURL(state)
	return url
}

func (h *AppHandler) SpotifyLogin(w http.ResponseWriter, r *http.Request) {

	fmt.Println("getting url and redirecting")
	url:= getSpotifyAuthLoginURL()
	
	
	fmt.Println("Redirect URL %s\n", url)
	http.Redirect(w, r, url, 301)
}

func (h *AppHandler) SpotifyCallback(w http.ResponseWriter, r *http.Request) {
	log.Printf("Callback hit \n")

	token, err := auth.Token(state, r)
	if err != nil {
		fmt.Println("Error with callback invalid")
		log.Fatal(err)
		w.WriteHeader(http.StatusForbidden)
	}
	// use the token to get an authenticated client
	client := auth.NewClient(token)
	user, userErr := client.CurrentUser()

	if userErr!=nil {
		log.Fatal(userErr)
		w.Write([]byte(fmt.Sprintf("%s",http.StatusForbidden)))
	}

	rUserErr, registeredUser:=h.UserService.FetchOrCreateUser(user, token)

	if rUserErr!=nil{
		log.Fatal(rUserErr)
		w.Write([]byte(fmt.Sprintf("%s",http.StatusInternalServerError)))
	}

	tokenString, tokenErr := h.TokenService.CreateToken(*registeredUser)

	if tokenErr != nil {
		log.Fatal(rUserErr)
		w.Write([]byte(fmt.Sprintf("%s",http.StatusInternalServerError)))
	}

	w.Header().Add("Auth", tokenString)
	json.NewEncoder(w).Encode(UserDto{
		UserId: registeredUser.UserId, 
		UserName: registeredUser.Username,
	})
}

func (h *AppHandler) SpotifyPlaylist(w http.ResponseWriter, r *http.Request){
	fmt.Print("sdf")
}

func (h *AppHandler) getUser(w http.ResponseWriter, r *http.Request){
	token := r.URL.Query().Get("token")

	if len(token)==0{
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(fmt.Sprintf("%s",http.StatusUnauthorized)))
	}

	claims, err := h.TokenService.ValidateToken(token)

	if err!=nil{
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(fmt.Sprintf("%s",http.StatusUnauthorized)))
	}


	user := h.UserService.FetchUser(claims.UserId)
	
	// client := auth.NewClient(user.SpotifyRefreshToken)
	// user, userErr := client.CurrentUser()

	// if userErr!=nil 
	// 	log.Fatal(userErr)
	// 	w.Write([]byte(fmt.Sprintf("%s",http.StatusForbidden)))
	// }

	json.NewEncoder(w).Encode(user)
}
