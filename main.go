package main

import (
	"log"
	"net/http"
	"os"
	"github.com/gorilla/mux"
	"github.com/gorilla/handlers"
)

func main() {
	initEvents()
	router := mux.NewRouter().StrictSlash(true)
	router.Use(contentJSONMiddleware)

	router.HandleFunc("/", homeLink)
	router.HandleFunc("/spotify-login", spotifyLogin)
	router.HandleFunc("/spotify-callback", spotifyCallback)
	router.HandleFunc("/spotify-playlist", spotifyPlaylist).Methods("GET")

	router.HandleFunc("/event", createEvent).Methods("POST")
	router.HandleFunc("/events/{id}", getOneEvent).Methods("GET")

	log.Fatal(http.ListenAndServe(":2580", handlers.CombinedLoggingHandler(os.Stdout, router)))

}

func contentJSONMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Add("Content-Type", "application/json")
        next.ServeHTTP(w, r)
    })
}