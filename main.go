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

	// Start the server
	fmt.Println("Server is running on https://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
