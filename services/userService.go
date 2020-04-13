package services

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/gofrs/uuid"
	"github.com/jinzhu/gorm"
	"github.com/nnajiabraham/spotube/config"
	"github.com/nnajiabraham/spotube/models"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)

//UserService ..
type UserService struct {
	DB *gorm.DB
	Config *config.Configs
}

//FetchUser fetches a user record
func (s *UserService) FetchUser(userID string) (*models.User) {
	registeredUser := &models.User{}
	
	s.DB.Where(&models.User{
		SpotifyID: userID,}).First(registeredUser)
	return registeredUser
}

//FetchOrCreateUser fetches a user record if exist or creates one
func (s *UserService) FetchOrCreateUser(user *spotify.PrivateUser, token *oauth2.Token) (*models.User, error) {

	registeredUser := &models.User{}
	
	//check if user or email is registered
	s.DB.Where(&models.User{
		SpotifyID: user.ID, 
		Email: user.Email}).First(registeredUser)

	if (models.User{}) != *registeredUser {
		registeredUser.SpotifyToken=token.AccessToken
		registeredUser.SpotifyRefreshToken=token.RefreshToken
		registeredUser.SpotifyTokenType=token.TokenType
		registeredUser.SpotifyTokenExpiry=strconv.FormatInt(token.Expiry.Unix(), 10)
		s.DB.Save(registeredUser)

		return registeredUser, nil
	}


	newUUID, err := uuid.NewV4()
	if err != nil {
		fmt.Printf("Something went wrong generating UUID: %s", err)
		return nil, err
	}
	
	fmt.Println("NEW USER REGISTERED")

	newUser := &models.User{
		UserID: newUUID.String(),
		Username: user.DisplayName, 
		Email: user.Email, 
		SpotifyID: user.ID, 
		SpotifyToken: token.AccessToken, 
		SpotifyRefreshToken: token.RefreshToken,
		SpotifyTokenType: token.TokenType,
		SpotifyTokenExpiry: strconv.FormatInt(token.Expiry.Unix(), 10)}

	s.DB.Create(newUser)

	return newUser, nil
}


//UpdateUser updates an existing user record
func (s *UserService) UpdateUser(user *spotify.PrivateUser, token *oauth2.Token) (*models.User, error) {

	registeredUser := &models.User{}
	
	//check if user or email is registered
	s.DB.Where(&models.User{
		SpotifyID: user.ID, 
		Email: user.Email}).First(registeredUser)

	if (models.User{}) == *registeredUser {
		userinfo := fmt.Sprintf("No User found with SpotifyId: %s and SpotifyEmail: %s", user.ID, user.Email)
		err:= errors.New(userinfo)
		return nil,err
	}	

	registeredUser.SpotifyToken=token.AccessToken
	registeredUser.SpotifyRefreshToken=token.RefreshToken
	registeredUser.SpotifyTokenType=token.TokenType
	registeredUser.SpotifyTokenExpiry=strconv.FormatInt(token.Expiry.Unix(), 10)
	s.DB.Save(registeredUser)
		
	return registeredUser, nil
}
