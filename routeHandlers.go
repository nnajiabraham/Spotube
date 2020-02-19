package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	// "github.com/zmb3/spotify"
)

type allEvents []event

var events = allEvents{
	{
		ID:          "1",
		Title:       "Introduction to Golang",
		Description: "Come join us for a chance to learn how golang works and get to eventually try it out",
	},
}

func createEvent(w http.ResponseWriter, r *http.Request) {
	var newEvent event
	reqBody, err := ioutil.ReadAll(r.Body)

	if err != nil {
		fmt.Fprintf(w, "Kindly enter data with the event title and description only in order to update")
	}
	
	json.Unmarshal(reqBody, &newEvent)

	events = append(events, newEvent)
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(newEvent)
}

func getOneEvent(w http.ResponseWriter, r *http.Request) {
	eventID := mux.Vars(r)["id"]

	for _, singleEvent := range events {
		if singleEvent.ID == eventID {
			json.NewEncoder(w).Encode(singleEvent)
		}
	}
}


func homeLink(w http.ResponseWriter, r *http.Request) {
	fmt.Println("hello")
	fmt.Fprintf(w, "Welcome home!")
}



func spotifyLogin(w http.ResponseWriter, r *http.Request) {

	fmt.Println("getting url and redirecting")
	url:= getSpotifyAuthLoginURL()

	http.Redirect(w, r, url, 301)
	

	// fmt.Fprintf(w, "Welcome home!"+scopes)
}

// func spotifyCallback(w http.ResponseWriter, r *http.Request) {
// 	fmt.Println("Welcome spotifyCallback!")

// 	token, err := auth.Token("state1", r)
//       if err != nil {
// 		  fmt.Printf("Err \n %s \n", err.Error())
//             http.Error(w, "Couldn't get token", http.StatusNotFound)
//             return
//       }
//       // create a client using the specified token
// 	  client := auth.NewClient(token)
// 	  userPlaylist, err := client.CurrentUsersPlaylists()
// 	  if err != nil{
// 		  fmt.Fprintf(w, "Unable to retrive playlist")
// 	  }

// 	  json.NewEncoder(w).Encode(userPlaylist)
	  
// }

func spotifyPlaylist(w http.ResponseWriter, r *http.Request){
	client := <-clientChannel
	token, err := client.Token()
	// userPlaylist, err := client.CurrentUsersPlaylists()

	if err != nil {
		log.Fatal(err)
	}
	json.NewEncoder(w).Encode(token)
	fmt.Print("sdf")
}

func initEvents() {
	events = allEvents{
		{
			ID:          "1",
			Title:       "Introduction to Golang",
			Description: "Come join us for a chance to learn how golang works and get to eventually try it out",
		},
	}
}