package services

import (
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/nnajiabraham/spotube/models"
)

type TokenService struct {
	
}

type Claims struct {
	Username string `json:"username"`
	UserId string `json:"userId"`
	SpotifyId string `json:"spotifyId"`
	jwt.StandardClaims
}

/* Set up a global string for our secret */
var mySigningKey = []byte("secret")

func (s *TokenService) CreateToken (user models.User) (string, error){	
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

func (s *TokenService) ValidateToken (token string) (Claims, error){
	claims := &Claims{}

	tkn, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return mySigningKey, nil
	})

	if err != nil || !tkn.Valid {
		return *claims, err
	}


	return *claims, nil
}