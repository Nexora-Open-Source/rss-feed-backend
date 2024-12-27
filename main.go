/*
Package main initializes the RSS Feed backend server.

This backend provides APIs to fetch and store RSS feed data and retrieve predefined feed sources.
It uses Google Cloud Datastore for storage and supports parsing RSS feeds via the `gofeed` library.

Key Features:
  - Fetch RSS feeds from a URL.
  - Store parsed feeds in Google Cloud Datastore.
  - Retrieve predefined or categorized RSS feed sources.
  - Designed for easy integration with frontend applications.

Run the application:

	$ go run main.go

Endpoints:
  - GET /fetch-store?url=<rss-url>: Fetch and store RSS feed data.
  - GET /feeds: Retrieve predefined RSS feed sources.
*/
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Nexora-Open-Source/rss-feed-backend/handlers"

	"github.com/gorilla/mux"
)

func main() {

	// Initialize the router
	router := mux.NewRouter()

	// Setup routes
	router.HandleFunc("/fetch-store", handlers.HandleFetchAndStore).Methods("GET")
	router.HandleFunc("/feeds", handlers.HandleGetFeeds).Methods("GET")

	// Attach the CORS middleware
	withCORS := CORSMiddleware(router)

	// Start the server
	fmt.Println("Server is running on https://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", withCORS))
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// If it's an OPTIONS request, we can respond with OK directly
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
