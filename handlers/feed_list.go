package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

// FeedSource represents a predefined RSS feed source
type FeedSource struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

/*
HandleGetFeeds returns a list of predefined RSS feed sources from a JSON file.

Example:

	GET /feeds

Response:
  - 200 OK: A JSON array of predefined feed sources.
*/
func HandleGetFeeds(w http.ResponseWriter, r *http.Request) {
	// Define the path to the JSON file
	filePath := "data/feeds.json"

	// Open the JSON file
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening feeds.json file: %v", err)
		http.Error(w, "Unable to load feeds", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Decode the JSON data
	var feeds []FeedSource
	if err := json.NewDecoder(file).Decode(&feeds); err != nil {
		log.Printf("Error decoding feeds.json file: %v", err)
		http.Error(w, "Invalid feeds format", http.StatusInternalServerError)
		return
	}

	// Respond with the list of feeds
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(feeds)
}
