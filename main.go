package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	handler "github.com/nnajiabraham/spotube/handlers"
	"github.com/nnajiabraham/spotube/models"
	"github.com/nnajiabraham/spotube/services"
)

// const (
//     dbPass 		= "password"
// 	dbHost 		= "localhost"
//     dbPort 		= "3306"
// 	dbUsername	= "root"
// )

func main() {
	println("starting")

	db, err := bootstrapDB()
	if err != nil {
		fmt.Println("err \n %s \n", err)
		panic("failed to connect database")
	}
	defer db.Close()

	userService := &services.UserService{DB: db}
	appHandler:= handler.AppHandlers{UserService: userService}

	// newuser := &models.User{User_id: time.Now().String(), 
	// 	Username: "testingFromMain",
	// 	Spotify_token: "string",
	// 	Spotify_refresh_token: "test",
	// }
	// userService.CreateUser(newuser)
	// user := userService.FetchUser("id")
	

	// u1 := uuid.Must(uuid.NewV4())

	router := mux.NewRouter().StrictSlash(true)
	router.Use(contentJSONMiddleware)

	router.HandleFunc("/", appHandler.HomeHandler)
	router.HandleFunc("/spotify-login", appHandler.SpotifyLogin)
	router.HandleFunc("/spotify-callback", appHandler.SpotifyCallback)
	router.HandleFunc("/spotify-playlist", appHandler.SpotifyPlaylist).Methods("GET")

	log.Println(http.ListenAndServe(":2580", handlers.CombinedLoggingHandler(os.Stdout, router)))

}

func contentJSONMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Add("Content-Type", "application/json")
        next.ServeHTTP(w, r)
    })
}

func bootstrapDB()(db *gorm.DB, err error){
	db, err = gorm.Open("mysql", "root:password@(localhost)/spotube?charset=utf8mb4&parseTime=True&loc=Local")
	if err != nil {
		fmt.Println("err \n %s \n", err)
		return nil, err
	}

	db.AutoMigrate(&models.User{})
	return db, nil
}