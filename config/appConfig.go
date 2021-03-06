package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/jinzhu/gorm"
	"github.com/joho/godotenv"
	"github.com/nnajiabraham/spotube/models"
)

// AppConfig .
type AppConfig struct {
}

// Configs .
type Configs struct{
	SPOTIFY_ID string
	SPOTIFY_SECRET string
	TOKEN_STATE string
	JWT_SIGNING_KEY string
	GOOGLE_CLIENT_ID string
	GOOGLE_CLIENT_SECRET string
}

// ReadConfig .
func (c *AppConfig) ReadConfig() (*Configs, error) {
	// loads values from .env into the system
    if err := godotenv.Load(); err != nil {
		return nil, errors.New("No .env file found missing important configs")
	}
	
	config := &Configs{
		TOKEN_STATE: os.Getenv("TOKEN_STATE"),
		SPOTIFY_ID: os.Getenv("SPOTIFY_ID"),
		SPOTIFY_SECRET: os.Getenv("SPOTIFY_SECRET"), 
		JWT_SIGNING_KEY: os.Getenv("JWT_SIGNING_KEY"),
		GOOGLE_CLIENT_ID: os.Getenv("GOOGLE_CLIENT_ID"),
		GOOGLE_CLIENT_SECRET: os.Getenv("GOOGLE_CLIENT_SECRET"),
	}
	
	return config, nil
}

// ConnectToDB .
func(c *AppConfig) ConnectToDB()(db *gorm.DB){
	db, err := gorm.Open("mysql", "root:password@(localhost)/spotube?charset=utf8mb4&parseTime=True&loc=Local")
	if err != nil {
		panic(fmt.Sprintf("failed to connect database: \n%s", err.Error()))
	}

	db.AutoMigrate(&models.User{})
	return db
}