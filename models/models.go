package models

import (
	"github.com/jinzhu/gorm"
)

type User struct{
	gorm.Model`json:"-"`
	UserID string`gorm:"primary_key;type:varchar(100);unique_index;not null" json:"userId"`
	Username string`gorm:"type:varchar(255);" json:"userName"`
	Email string`gorm:"type:varchar(100);unique_index" json:"email"`
	SpotifyID string`gorm:"type:varchar(100);unique_index" json:"spotifyId"`
	SpotifyToken string`gorm:"type:varchar(255);" json:"-"`
	SpotifyRefreshToken string`gorm:"type:varchar(255);" json:"-"`
	SpotifyTokenType string`gorm:"type:varchar(255);" json:"-"`
	SpotifyTokenExpiry string`gorm:"type:varchar(255);" json:"-"`
}