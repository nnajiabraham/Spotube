package services

import (
	"fmt"

	"github.com/gofrs/uuid"
	"github.com/jinzhu/gorm"
	"github.com/nnajiabraham/spotube/models"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)

type UserService struct {
	DB *gorm.DB
}

func (s *UserService) FetchUser(userId string) (*models.User) {
	registeredUser := &models.User{}
	
	s.DB.Where(&models.User{
		SpotifyId: userId,}).First(registeredUser)
	return registeredUser
}

func (s *UserService) FetchOrCreateUser(user *spotify.PrivateUser, token *oauth2.Token) (error, *models.User) {

	registeredUser := &models.User{}
	
	//check if user or email is registered
	s.DB.Where(&models.User{
		SpotifyId: user.ID, 
		Email: user.Email}).First(registeredUser)

		// t,_:=time.Parse("2020-04-04 03:01:07.440281", time.Now().String())

	if (models.User{}) != *registeredUser {
		registeredUser.SpotifyToken=token.AccessToken
		registeredUser.SpotifyRefreshToken=token.RefreshToken
		registeredUser.SpotifyTokenType=token.TokenType
		registeredUser.SpotifyTokenExpiry=token.Expiry.String()
		s.DB.Save(registeredUser)

		
		return nil, registeredUser
	}


	newUUID, err := uuid.NewV4()
	if err != nil {
		fmt.Printf("Something went wrong generating UUID: %s", err)
		return err, nil
	}
	
	fmt.Println("NEW USER REGISTERED")

	newUser := &models.User{
	UserId: newUUID.String(),
	Username: user.DisplayName, 
	Email: user.Email, 
	SpotifyId: user.ID, 
	SpotifyToken: token.AccessToken, 
	SpotifyRefreshToken: token.RefreshToken,
	SpotifyTokenType: token.TokenType,
	SpotifyTokenExpiry: token.Expiry.String()}

	s.DB.Create(newUser)

	return nil,newUser
}

