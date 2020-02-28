package services

import (
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

func (s *UserService) CreateUser(user *models.User) (error) {

	result :=s.DB.Create(user)

	if result.Error!= nil {
		return result.Error
	}

	return nil
}

/* Set up a global string for our secret */
// var mySigningKey = []byte("secret")

//   /* Handlers */
// func (s *UserService) CreateToken (user *models.User){
//       /* Create the token */
//     token := jwt.New(jwt.SigningMethodHS256)

//     /* Create a map to store our claims */
//     claims := token.Claims.(jwt.MapClaims)

//     /* Set token claims */
//     claims["admin"] = true
//     claims["name"] = "Ado Kukic"
//     claims["exp"] = time.Now().Add(time.Hour * 24).Unix()

//     /* Sign the token with our secret */
//     tokenString, _ := token.SignedString(mySigningKey)
// }

