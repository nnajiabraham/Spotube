package routes

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/nnajiabraham/spotube/config"
	"github.com/nnajiabraham/spotube/models"
	"github.com/nnajiabraham/spotube/services"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)
 
type AppHandler struct{
	UserService *services.UserService
	TokenService *services.TokenService
	SpotifyService *services.SpotifyService
	Config *config.Configs
}

type ErrorDto struct{
	Status int
	Message string
}

type claimKeyType string

const claimKey claimKeyType = "claims"

func contentJSONMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Add("Content-Type", "application/json")
        next.ServeHTTP(w, r)
    })
}

func (h *AppHandler) verifyJWT(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		token, err := r.Cookie("token")

		if err != nil {
			if err == http.ErrNoCookie {
				log.Printf("unauthorized: %s ",err.Error())
				// If the cookie is not set, return an unauthorized status
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(ErrorDto{
					Status: http.StatusUnauthorized, 
					Message: "Unauthorized",
				})
				return
			}
			// For any other type of error, return a bad request status
			log.Printf("StatusBadRequest: %s ",err.Error())
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorDto{
					Status: http.StatusBadRequest, 
					Message: "StatusBadRequest",
			})
			return
		}

		claims, err := h.TokenService.ValidateToken(token.Value)

		if err!=nil{
			log.Printf("Error validating token/claims: %s ",err.Error())
			w.WriteHeader(http.StatusUnauthorized)	
			json.NewEncoder(w).Encode(ErrorDto{
				Status: http.StatusUnauthorized, 
				Message: "Unauthorized",
			})
			return
		}


		ctx := context.WithValue(r.Context(), claimKey, claims)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// RegisterRoutes registers all routes paths with handlers.
func (h *AppHandler) RegisterRoutes(router *mux.Router) {
	router.Use(contentJSONMiddleware)
	router.HandleFunc("/", h.HomeHandler)
	router.HandleFunc("/spotify-login", h.SpotifyLogin)
	router.HandleFunc("/spotify-callback", h.SpotifyCallback)

	protectedRoutes := router.NewRoute().Subrouter()
	protectedRoutes.Use(h.verifyJWT)
	protectedRoutes.HandleFunc("/spotify-playlist", h.SpotifyPlaylist).Methods("GET")
	protectedRoutes.HandleFunc("/user", h.GetUserProfile)
}

//npm install -g localtunnel
//npx localtunnel --port 8000
//lt --port 2580 --subdomain nnajiabraham 
// lt -h "http://serverless.social" --port 2580 --open true --subdomain nnajiabraham
// lt -h "http://viewshd.com" --port 2580 --subdomain nnajiabraham

func (h *AppHandler) HomeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome home from new handler!")
}

func (h *AppHandler) SpotifyLogin(w http.ResponseWriter, r *http.Request) {

	url:= h.SpotifyService.GetSpotifyAuthLoginURL()
	
	fmt.Printf("Login Redirect URL %s\n", url)
	http.Redirect(w, r, url, 301)
}

func (h *AppHandler) SpotifyCallback(w http.ResponseWriter, r *http.Request) {

	client, err:= h.SpotifyService.GetSpotifyClientToken(r)
	if err != nil {
		log.Printf("Spotify login callback: %s ",err.Error())
		w.WriteHeader(http.StatusUnauthorized)	
		json.NewEncoder(w).Encode(ErrorDto{
			Status: http.StatusUnauthorized, 
			Message: "Unauthorized",
		})
	}

	user, userErr := client.SpotifyClient.CurrentUser()
	if userErr!=nil {
		log.Printf("Spotify User Not Found: %s ",userErr.Error())
		w.WriteHeader(http.StatusInternalServerError)	
		json.NewEncoder(w).Encode(ErrorDto{
			Status: http.StatusInternalServerError, 
			Message: "Something went wrong with fetching user info",
		})
        return
	}

	registeredUserErr, registeredUser:=h.UserService.FetchOrCreateUser(user, client.UserToken)
	if registeredUserErr!=nil{
		log.Printf("Unable to fetch or create user: %s ",registeredUserErr.Error())
		w.WriteHeader(http.StatusInternalServerError)	
		json.NewEncoder(w).Encode(ErrorDto{
			Status: http.StatusInternalServerError, 
			Message: "Something went wrong with fetching user info",
		})
        return
	}

	expirationTime := time.Now().Add(time.Hour * 24)

	jwtString, jwtErr := h.TokenService.CreateToken(registeredUser, expirationTime)

	if jwtErr != nil {
		log.Printf("Unable to create token for user: %s ",jwtErr.Error())
		w.WriteHeader(http.StatusInternalServerError)	
		json.NewEncoder(w).Encode(ErrorDto{
			Status: http.StatusInternalServerError, 
			Message: "Something went wrong with fetching user info",
		})
        return
	}

	w.Header().Add("Auth", jwtString)

	http.SetCookie(w, &http.Cookie{
		Name:    "token",
		Value:   jwtString,
		Expires: expirationTime,
	})

	json.NewEncoder(w).Encode(models.User{
		UserID: registeredUser.UserID,
		SpotifyID: registeredUser.SpotifyID, 
		Username: registeredUser.Username,
		Email: registeredUser.Email,
	})
}

func (h *AppHandler) SpotifyPlaylist(w http.ResponseWriter, r *http.Request){
	fmt.Print("sdf")
}

func (h *AppHandler) GetUserProfile(w http.ResponseWriter, r *http.Request){

	claims := r.Context().Value(claimKey).(services.Claims)
	user := h.UserService.FetchUser(claims.SpotifyId)

	tokenExpTime, timeParseErr:= strconv.ParseInt(user.SpotifyTokenExpiry, 10, 64)
	if timeParseErr != nil {
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
		json.NewEncoder(w).Encode(models.User{
			UserID: user.UserID, 
			SpotifyID: user.SpotifyID,
			Username: user.Username,
			Email: user.Email,
		})
	}

	client:= h.SpotifyService.GetSpotifyAuth().NewClient(userOauthToken)
	userSpotifyProfile, userErr := client.CurrentUser()

	if userErr!=nil {
		log.Printf("Spotify User Not Found: %s ",userErr.Error())
		w.WriteHeader(http.StatusInternalServerError)	
		json.NewEncoder(w).Encode(ErrorDto{
			Status: http.StatusInternalServerError, 
			Message: "StatusUnauthorized",
		})
	}

	updatedUser, updateUserErr := h.UserService.UpdateUser(userSpotifyProfile, userOauthToken)
	
	if updateUserErr!=nil {
		log.Printf("Err Updating User: %s ",updateUserErr.Error())
		w.WriteHeader(http.StatusUnauthorized)	
		json.NewEncoder(w).Encode(ErrorDto{
			Status: http.StatusUnauthorized, 
			Message: "StatusUnauthorized",
		})
	}

	fmt.Println("UPDATED USER TOKEN")
	json.NewEncoder(w).Encode(models.User{
			UserID: updatedUser.UserID, 
			SpotifyID: updatedUser.SpotifyID,
			Username: updatedUser.Username,
			Email: updatedUser.Email,
	})
}
