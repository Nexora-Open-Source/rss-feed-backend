package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/rss-feed-backend/utils"

	"cloud.google.com/go/datastore"
	"github.com/joho/godotenv"
)

// Datastore client instance
var datastoreClient *datastore.Client

func init() {
	var err error

	// Load environment variables from .env file
	err = godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	projectID := os.Getenv("PROJECT_ID")

	datastoreClient, err = datastore.NewClient(context.Background(), projectID)
	if err != nil {
		panic(fmt.Sprintf("Failed to create Datastore client: %v", err))
	}
}

// HandleFetchAndStore handles fetching and storing RSS feeds
func HandleFetchAndStore(w http.ResponseWriter, r *http.Request) {

	// Get the RSS feed URL from querty params
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, "URL parameter is missing", http.StatusBadRequest)
		return
	}

	// Parse the RSS feed
	feedItems, err := utils.FetchRSSFeed(url)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch RSS feed: %v", err), http.StatusInternalServerError)
		return
	}

	// Save the feed items to Datastore
	if err := SaveToDatastore(feedItems); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save to Datastore: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Feed items saved successfully")
}
