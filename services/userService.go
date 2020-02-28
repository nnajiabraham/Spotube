package services

import (
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/jinzhu/gorm"
	"github.com/nnajiabraham/spotube/models"
)

type UserService struct {
	DB *gorm.DB
}

func (s *UserService) FetchUser(userId string) (*models.User) {
	var user models.User
	
	s.DB.First(&user, "user_id = ?", userId)
	return &user
}

func (s *UserService) FetchOrCreateUser(user *models.User) (error) {
	result:= s.DB.FirstOrCreate(user, models.User{SpotifyId: user.SpotifyId})
	
	if result.Error!= nil {
		return result.Error
	}

	return nil
}

type Claims struct {
	Username string `json:"username"`
	UserId string `json:"userId"`
	SpotifyId string `json:"spotifyId"`
	jwt.StandardClaims
}

/* Set up a global string for our secret */
var mySigningKey = []byte("secret")

  /* Handlers */
func (s *UserService) CreateToken (user models.User) (string, error){
	
	expirationTime := time.Now().Add(time.Hour * 24).Unix()

	// Create the JWT claims, which includes the username and expiry time
	claims := &Claims{
		UserId: user.UserId,
		SpotifyId: user.SpotifyId,
		Username: user.Username,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime,
		},
	}

	// Declare the token with the algorithm used for signing, and the claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

    /* Sign the token with our secret */
	tokenString,err := token.SignedString(mySigningKey)

	 if err!=nil {
		return "", err
	 }

	 return tokenString, nil
}

