package routes

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/nnajiabraham/spotube/config"
	"github.com/nnajiabraham/spotube/models"
	"github.com/nnajiabraham/spotube/services"
	"github.com/zmb3/spotify"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)
 
type AppHandler struct{
	UserService *services.UserService
	TokenService *services.TokenService
	SpotifyService *services.SpotifyService
	Config *config.Configs
}

type response struct {
	StatusCode int        `json:"statusCode"`
	Data    interface{} `json:"response"`
}

type claimKeyType string

const claimKey claimKeyType = "claims"

func (h *AppHandler) verifyJWT(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		token, err := r.Cookie("token")

		if err != nil {
			if err == http.ErrNoCookie {
				log.Printf("unauthorized: %s ",err.Error())
				// If the cookie is not set, return an unauthorized status
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(response{
					StatusCode: http.StatusUnauthorized, 
					Data: "Unauthorized",
				})
				return
			}
			
			// For any other type of error, return a bad request status
			log.Printf("StatusBadRequest: %s ",err.Error())
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response{
					StatusCode: http.StatusBadRequest, 
					Data: "StatusBadRequest",
			})
			return
		}

		claims, err := h.TokenService.ValidateToken(token.Value)

		if err!=nil{
			log.Printf("Error validating token/claims: %s ",err.Error())
			w.WriteHeader(http.StatusUnauthorized)	
			json.NewEncoder(w).Encode(response{
				StatusCode: http.StatusUnauthorized, 
				Data: "Unauthorized",
			})
			return
		}

		ctx := context.WithValue(r.Context(), claimKey, claims)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func contentJSONMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Add("Content-Type", "application/json")
        next.ServeHTTP(w, r)
    })
}

func responseHandler(handler func(w http.ResponseWriter, r *http.Request) (interface{}, int, error)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		data, status, err := handler(w, r)

		if err != nil {
			data = err.Error()
		}

		w.WriteHeader(status)
		w.Header().Set("Content-Type", "application/json")

		err = json.NewEncoder(w).Encode(response{Data: data, StatusCode: status})
		if err != nil {
			log.Printf("could not encode response to output: %v", err)
		}
	}
}

// RegisterRoutes registers all routes paths with handlers.
func (h *AppHandler) RegisterRoutes(router *mux.Router) {
	router.Use(contentJSONMiddleware)
	router.HandleFunc("/", h.homeHandler)
	router.HandleFunc("/spotify-login", h.spotifyLogin)
	router.HandleFunc("/spotify-callback", responseHandler(h.spotifyCallback))

	protectedRoutes := router.NewRoute().Subrouter()
	protectedRoutes.Use(h.verifyJWT)
	protectedRoutes.HandleFunc("/spotify-playlist", responseHandler(h.getSpotifyPlaylist)).Methods("GET")
	protectedRoutes.HandleFunc("/user", responseHandler(h.getUserProfile))
}

//npm install -g localtunnel
//npx localtunnel --port 8000
//lt --port 2580 --subdomain nnajiabraham 
// lt -h "http://serverless.social" --port 2580 --open true --subdomain nnajiabraham
// lt -h "http://viewshd.com" --port 2580 --subdomain nnajiabraham


func (h *AppHandler) homeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "No place like home")
}

func (h *AppHandler) spotifyLogin(w http.ResponseWriter, r *http.Request) {

	url:= h.SpotifyService.GetSpotifyAuthLoginURL()
	
	fmt.Printf("Login Redirect URL %s\n", url)
	http.Redirect(w, r, url, 301)
}

func (h *AppHandler) spotifyCallback(w http.ResponseWriter, r *http.Request) (interface{}, int, error){

	client, err:= h.SpotifyService.GetSpotifyClientToken(r)
	if err != nil {
		log.Printf("Spotify login callback: %s ",err.Error())
		return nil, http.StatusUnauthorized, errors.New("Unauthorized")
	}

	user, userErr := client.SpotifyClient.CurrentUser()
	if userErr!=nil {
		log.Printf("Spotify User Not Found: %s ",userErr.Error())
        return nil, http.StatusNotFound, errors.New("Spotify User Not Found")
	}

	registeredUserErr, registeredUser:=h.UserService.FetchOrCreateUser(user, client.UserToken)
	if registeredUserErr!=nil{
		log.Printf("Unable to fetch or create user: %s ",registeredUserErr.Error())
        return nil, http.StatusInternalServerError, errors.New("Internal Server Error")
	}

	expirationTime := time.Now().Add(time.Hour * 24)

	jwtString, jwtErr := h.TokenService.CreateToken(registeredUser, expirationTime)

	if jwtErr != nil {
		log.Printf("Unable to create token for user: %s ",jwtErr.Error())
        return nil, http.StatusInternalServerError, errors.New("Internal Server Error")
	}

	// w.Header().Add("Auth", jwtString)

	http.SetCookie(w, &http.Cookie{
		Name:    "token",
		Value:   jwtString,
		Expires: expirationTime,
	})

    return models.User{
		UserID: registeredUser.UserID,
		SpotifyID: registeredUser.SpotifyID, 
		Username: registeredUser.Username,
		Email: registeredUser.Email,
	}, http.StatusOK, nil
}

func (h *AppHandler) getSpotifyPlaylist(w http.ResponseWriter, r *http.Request) (interface{}, int, error) {
	claims := r.Context().Value(claimKey).(services.Claims)
	user := h.UserService.FetchUser(claims.SpotifyId)

	userOauthToken, userOauthTokenErr := createSpotifyUserToken(user)
	if userOauthTokenErr!=nil {
		log.Printf("Unable to get token: %s ",userOauthTokenErr.Error())
		return nil, http.StatusInternalServerError, errors.New("Internal Server Error")
	}

	client:= h.SpotifyService.GetSpotifyAuth().NewClient(userOauthToken)

	offset, limit := 0, 20
	
	options := &spotify.Options{
		Offset: &offset, 
		Limit: &limit,
	}

	userPlaylist := []spotify.SimplePlaylist{}

	initialPlaylist, initialPlaylistErr := client.CurrentUsersPlaylistsOpt(options)

	if initialPlaylistErr != nil{
		log.Printf("Unable to get users playlist: %s ",initialPlaylistErr.Error())
		return nil, http.StatusInternalServerError, errors.New("Internal Server Error")
	}

	for _, playlist := range initialPlaylist.Playlists{
		userPlaylist = append(userPlaylist, playlist)
	}

	if initialPlaylist.Total <= 20 {
		return userPlaylist, http.StatusOK, nil
	}

	noOfPlaylistPages:= int(math.Ceil(float64(initialPlaylist.Total) / float64(limit)))

	for page:=1; page<noOfPlaylistPages;{
		page++
		nextOffset := (limit*page)-limit
		options.Offset = &nextOffset
		nextPlaylists, nextPlaylistsErr := client.CurrentUsersPlaylistsOpt(options)

		if nextPlaylistsErr != nil{
			log.Printf("Unable to get users playlist: %s ",nextPlaylistsErr.Error())
			return nil, http.StatusInternalServerError, errors.New("Internal Server Error")
		}

		for _, playlist := range nextPlaylists.Playlists{
			userPlaylist = append(userPlaylist, playlist)
		}
	}

	return userPlaylist, http.StatusOK, nil
}

func (h *AppHandler) getUserProfile(w http.ResponseWriter, r *http.Request) (interface{}, int, error){

	claims := r.Context().Value(claimKey).(services.Claims)
	user := h.UserService.FetchUser(claims.SpotifyId)

	userOauthToken, userOauthTokenErr := createSpotifyUserToken(user)
	if userOauthTokenErr!=nil {
		log.Printf("Unable to get token: %s ",userOauthTokenErr.Error())
		return nil, http.StatusInternalServerError, errors.New("Internal Server Error")
	}

	if userOauthToken.Valid() {
		return models.User{
			UserID: user.UserID, 
			SpotifyID: user.SpotifyID,
			Username: user.Username,
			Email: user.Email,
		}, http.StatusOK, nil
	}

	client:= h.SpotifyService.GetSpotifyAuth().NewClient(userOauthToken)
	userSpotifyProfile, userErr := client.CurrentUser()

	if userErr!=nil {
		log.Printf("Spotify User Not Found: %s ",userErr.Error())
		return nil, http.StatusInternalServerError, errors.New("Internal Server Error")
	}

	updatedUser, updateUserErr := h.UserService.UpdateUser(userSpotifyProfile, userOauthToken)
	
	if updateUserErr!=nil {
		log.Printf("Err Updating User: %s ",updateUserErr.Error())
		return nil, http.StatusInternalServerError, errors.New("Internal Server Error")
	}

	return models.User{
			UserID: updatedUser.UserID, 
			SpotifyID: updatedUser.SpotifyID,
			Username: updatedUser.Username,
			Email: updatedUser.Email,
	}, http.StatusOK, nil
}

func createSpotifyUserToken(user *models.User) (*oauth2.Token, error){
	tokenExpTime, timeParseErr:= strconv.ParseInt(user.SpotifyTokenExpiry, 10, 64)

	if timeParseErr != nil {
		log.Printf("Error parsing time to oauth2token type")
		return nil, timeParseErr
	}
	
	return &oauth2.Token{
		Expiry: time.Unix(tokenExpTime, 0),
		TokenType: user.SpotifyTokenType,
		AccessToken: user.SpotifyToken,
		RefreshToken: user.SpotifyRefreshToken,
	}, nil
}