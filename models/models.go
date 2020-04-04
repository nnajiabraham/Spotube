package models

import (
	"github.com/jinzhu/gorm"
)

type User struct{
	gorm.Model
	UserId string`gorm:"primary_key;type:varchar(100);unique_index;not null"`
	Username string
	Email string`gorm:"type:varchar(100);unique_index"`
	SpotifyId string`gorm:"type:varchar(100);unique_index"`
	SpotifyToken string`gorm:"type:varchar(255);"`
	SpotifyRefreshToken string`gorm:"type:varchar(255);"`
	SpotifyTokenType string`gorm:"type:varchar(255);"`
	SpotifyTokenExpiry string`gorm:"type:varchar(255);"`
}