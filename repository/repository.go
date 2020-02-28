package repository

import (
	"github.com/nnajiabraham/spotube/models"
)

type UserRepo interface {
	FetchUser(user_id string) (*models.User)
	CreateUser(user *models.User) (error)
	UpdateUser(user *models.User) (error)
}