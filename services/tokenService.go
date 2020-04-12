package services

import (
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/nnajiabraham/spotube/config"
	"github.com/nnajiabraham/spotube/models"
)

type TokenService struct {
	Config *config.Configs
}

type Claims struct {
	Username string `json:"username"`
	UserId string `json:"userId"`
	SpotifyId string `json:"spotifyId"`
	jwt.StandardClaims
}

func (s *TokenService) getSigningKey() []byte{
	return []byte(s.Config.JWT_SIGNING_KEY)
}

func (s *TokenService) CreateToken (user *models.User, expirationTime time.Time) (string, error){	

	// Create the JWT claims, which includes the username and expiry time
	claims := &Claims{
		UserId: user.UserID,
		SpotifyId: user.SpotifyID,
		Username: user.Username,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	// Declare the token with the algorithm used for signing, and the claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

    /* Sign the token with our secret */
	tokenString,err := token.SignedString(s.getSigningKey())

	if err!=nil {
	return "", err
	}

	return tokenString, nil
}

func (s *TokenService) ValidateToken (token string) (Claims, error){
	claims := &Claims{}

	tkn, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		return s.getSigningKey(), nil
	})

	if err != nil || !tkn.Valid {
		return *claims, err
	}


	return *claims, nil
}