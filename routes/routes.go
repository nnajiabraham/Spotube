package routes

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/nnajiabraham/spotube/services"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)
 
type AppHandler struct{
	UserService *services.UserService
	TokenService *services.TokenService
}

type UserDto struct{
	UserId string
	UserName string
}

type ErrorDto struct{
	Status int
	Message string
}

var (
	scopes					= "user-read-private user-read-email playlist-read-private playlist-read-collaborative"
	redirectURICallback		= "http://nnajiabraham.viewshd.com/spotify-callback"
	state = os.Getenv("TOKEN_STATE") 
	// clientChannel    = make(chan *spotify.Client)
	auth = spotify.NewAuthenticator(redirectURICallback, scopes)
)

// RegisterRoutes registers all routes paths with handlers.
func (h *AppHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/", h.HomeHandler)
	router.HandleFunc("/spotify-login", h.SpotifyLogin)
	router.HandleFunc("/spotify-callback", h.SpotifyCallback)
	router.HandleFunc("/spotify-playlist", h.SpotifyPlaylist).Methods("GET")
	router.HandleFunc("/user", h.GetUser)
}

//npm install -g localtunnel
//npx localtunnel --port 8000
//lt --port 2580 --subdomain nnajiabraham 
// lt -h "http://serverless.social" --port 2580 --open true --subdomain nnajiabraham
// lt -h "http://viewshd.com" --port 2580 --subdomain nnajiabraham

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
	
	
	fmt.Printf("Redirect URL %s\n", url)
	http.Redirect(w, r, url, 301)
}

func (h *AppHandler) SpotifyCallback(w http.ResponseWriter, r *http.Request) {

	token, err := auth.Token(state, r)
	if err != nil {
		log.Printf("Spotify login callback: %s ",err.Error())
		w.WriteHeader(http.StatusUnauthorized)	
		json.NewEncoder(w).Encode(ErrorDto{
			Status: http.StatusUnauthorized, 
			Message: "Unauthorized",
		})
        return
	}
	// use the token to get an authenticated client
	client := auth.NewClient(token)
	user, userErr := client.CurrentUser()

	if userErr!=nil {
		log.Printf("Spotify User Not Found: %s ",userErr.Error())
		w.WriteHeader(http.StatusInternalServerError)	
		json.NewEncoder(w).Encode(ErrorDto{
			Status: http.StatusInternalServerError, 
			Message: "Something went wrong with fetching user info",
		})
        return
	}

	rUserErr, registeredUser:=h.UserService.FetchOrCreateUser(user, token)

	if rUserErr!=nil{
		log.Printf("Unable to fetch or create user: %s ",rUserErr.Error())
		w.WriteHeader(http.StatusInternalServerError)	
		json.NewEncoder(w).Encode(ErrorDto{
			Status: http.StatusInternalServerError, 
			Message: "Something went wrong with fetching user info",
		})
        return
	}

	tokenString, tokenErr := h.TokenService.CreateToken(registeredUser)

	if tokenErr != nil {
		log.Printf("Unable to create token for user: %s ",tokenErr.Error())
		w.WriteHeader(http.StatusInternalServerError)	
		json.NewEncoder(w).Encode(ErrorDto{
			Status: http.StatusInternalServerError, 
			Message: "Something went wrong with fetching user info",
		})
        return
	}

	w.Header().Add("Auth", tokenString)

	http.SetCookie(w, &http.Cookie{
		Name:    "token",
		Value:   tokenString,
		// Expires: tokenString,
	})
	json.NewEncoder(w).Encode(UserDto{
		UserId: registeredUser.UserId, 
		UserName: registeredUser.Username,
	})
}

func (h *AppHandler) SpotifyPlaylist(w http.ResponseWriter, r *http.Request){
	fmt.Print("sdf")
}

func (h *AppHandler) GetUser(w http.ResponseWriter, r *http.Request){
	fmt.Println("Getting User")
	token := r.URL.Query()["token"][0]
	fmt.Println("Token Gotten")
	fmt.Printf("Token %s\n", token)

	if len(token)==0{
		log.Printf("Empty token tried authenticating: %s ",token)
		w.WriteHeader(http.StatusUnauthorized)	
		json.NewEncoder(w).Encode(ErrorDto{
			Status: http.StatusUnauthorized, 
			Message: "Unauthorized",
		})
        return
	}

	claims, err := h.TokenService.ValidateToken(token)

	if err!=nil{
		log.Printf("Error validating token/claims: %s ",err.Error())
		w.WriteHeader(http.StatusUnauthorized)	
		json.NewEncoder(w).Encode(ErrorDto{
			Status: http.StatusUnauthorized, 
			Message: "Unauthorized",
		})
        return
	}

	user := h.UserService.FetchUser(claims.SpotifyId)

	tokenExpTime, timeParseErr:= strconv.ParseInt(user.SpotifyTokenExpiry, 10, 64)
	if err != nil {
		log.Printf("Error parsing time to oauth2token type : %s ",timeParseErr.Error())
		w.WriteHeader(http.StatusInternalServerError)	
		json.NewEncoder(w).Encode(ErrorDto{
			Status: http.StatusInternalServerError,
			Message: "Internal Server Error",
		})
        return
	}
	
	userOauthToken :=  &oauth2.Token{
		Expiry: time.Unix(tokenExpTime, 0),
		TokenType: user.SpotifyTokenType,
		AccessToken: user.SpotifyToken,
		RefreshToken: user.SpotifyRefreshToken,
	}

	if userOauthToken.Valid() {
		getSpotifyAuthLoginURL()
		client:= auth.NewClient(userOauthToken)
		playlist, err:= client.CurrentUsersPlaylists()
		if err!=nil{
			json.NewEncoder(w).Encode(user)
		}
		json.NewEncoder(w).Encode(playlist)
		return ;
	}

	getSpotifyAuthLoginURL()
	client:= auth.NewClient(userOauthToken)
	reAuthUser, userErr := client.CurrentUser()

	if userErr!=nil {
		log.Printf("Spotify User Not Found: %s ",userErr.Error())
		w.WriteHeader(http.StatusInternalServerError)	
		json.NewEncoder(w).Encode(ErrorDto{
			Status: http.StatusInternalServerError, 
			Message: "StatusUnauthorized",
		})
        return
	}

	updateUserErr, updatedUser := h.UserService.UpdateUser(reAuthUser, userOauthToken)
	
	if updateUserErr!=nil {
		log.Printf("Err Updating User: %s ",updateUserErr.Error())
		w.WriteHeader(http.StatusUnauthorized)	
		json.NewEncoder(w).Encode(ErrorDto{
			Status: http.StatusUnauthorized, 
			Message: "StatusUnauthorized",
		})
        return
	}
	playlist, err:= client.CurrentUsersPlaylists()
	if err!=nil{
		json.NewEncoder(w).Encode(user)
	}
	fmt.Println(updatedUser)
	fmt.Println("UPDATED USER TOKEN")
	json.NewEncoder(w).Encode(playlist)
}
