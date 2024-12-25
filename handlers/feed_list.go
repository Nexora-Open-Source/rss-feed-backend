package handlers

import (
	"encoding/json"
	"net/http"
)

// FeedSource represents a predefined RSS feed source
type FeedSource struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// HandleGetFeeds provides a list of predefined RSS feeds
func HandleGetFeeds(w http.ResponseWriter, r *http.Request) {

	feeds := []FeedSource{
		{Name: "TechCrunch", URL: "https://techcrunch.com/feed/"},
		{Name: "BBC News", URL: "http://feeds.bbci.co.uk/news/rss.xml"},
		{Name: "The Verge", URL: "https://www.theverge.com/rss/index.xml"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(feeds)
}
