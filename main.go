package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/nnajiabraham/spotube/config"
	"github.com/nnajiabraham/spotube/routes"
	"github.com/nnajiabraham/spotube/services"
)

func main() {	
	config := &config.Config{}
	configs, err:= config.ReadConfig()
	db := config.ConnectToDB()

	if err != nil{
		panic(fmt.Sprintf("Startup issues: \n%s", err.Error()))
	}
	
	defer db.Close()

	spotifyService := &services.SpotifyService{Config: configs}
	tokenService := &services.TokenService{Config: configs}
	userService := &services.UserService{DB: db, Config: configs}
	appHandler:= routes.AppHandler{
		UserService: userService,
		TokenService: tokenService, 
		SpotifyService: spotifyService,
		Config: configs,
	}

	router := mux.NewRouter().StrictSlash(true)
	appHandler.RegisterRoutes(router)

	log.Println(http.ListenAndServe(":2580", handlers.CombinedLoggingHandler(os.Stdout, router)))
}




