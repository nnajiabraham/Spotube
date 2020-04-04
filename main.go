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
	"github.com/joho/godotenv"
	"github.com/nnajiabraham/spotube/models"
	"github.com/nnajiabraham/spotube/routes"
	"github.com/nnajiabraham/spotube/services"
)

func main() {
    // loads values from .env into the system
    if err := godotenv.Load(); err != nil {
        log.Print("No .env file found")
    }
	
	db, err := connectToDB()
	if err != nil {
		panic("failed to connect database")
	}
	defer db.Close()

	userService := &services.UserService{DB: db}
	tokenService := &services.TokenService{}
	appHandler:= routes.AppHandler{
		UserService: userService,
		TokenService: tokenService,
	}

	router := mux.NewRouter().StrictSlash(true)
	router.Use(contentJSONMiddleware)
	router.
	appHandler.RegisterRoutes(router);

	log.Println(http.ListenAndServe(":2580", handlers.CombinedLoggingHandler(os.Stdout, router)))
}

func contentJSONMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Add("Content-Type", "application/json")
        next.ServeHTTP(w, r)
    })
}

func connectToDB()(db *gorm.DB, err error){
	db, err = gorm.Open("mysql", "root:password@(localhost)/spotube?charset=utf8mb4&parseTime=True&loc=Local")
	if err != nil {
		fmt.Println("err", err.Error())
		return nil, err
	}

	db.AutoMigrate(&models.User{})
	return db, nil
}
