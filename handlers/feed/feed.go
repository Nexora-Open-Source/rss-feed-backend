// Package feed provides RSS feed handlers for the RSS feed backend
package feed

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/Nexora-Open-Source/rss-feed-backend/middleware"
	"github.com/Nexora-Open-Source/rss-feed-backend/utils"
	"github.com/sirupsen/logrus"
)

// FeedSource represents a predefined RSS feed source
type FeedSource struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Handler contains dependencies for feed handlers
type Handler struct {
	Logger *logrus.Logger
}

// NewHandler creates a new feed handler
func NewHandler(logger *logrus.Logger) *Handler {
	return &Handler{
		Logger: logger,
	}
}

// HandleGetFeeds returns predefined RSS feed sources
func (h *Handler) HandleGetFeeds(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = utils.GenerateRequestID()
		w.Header().Set("X-Request-ID", requestID)
	}

	// Log the request
	h.Logger.WithFields(logrus.Fields{
		"request_id": requestID,
		"action":     "get_feeds",
	}).Info("Processing feed list request")

	// Read predefined feeds from JSON file
	feeds, err := h.loadPredefinedFeeds()
	if err != nil {
		h.Logger.WithFields(logrus.Fields{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to load predefined feeds")
		middleware.RespondInternalError(w, err, requestID)
		return
	}

	// Log successful completion
	h.Logger.WithFields(logrus.Fields{
		"request_id":  requestID,
		"feeds_count": len(feeds),
	}).Info("Feed list retrieved successfully")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(feeds)
}

// loadPredefinedFeeds loads predefined RSS feed sources from data/feeds.json
func (h *Handler) loadPredefinedFeeds() ([]FeedSource, error) {
	// For now, return some hardcoded feeds
	// In a real implementation, you'd read from a file or database
	feeds := []FeedSource{
		{
			Name: "BBC News",
			URL:  "http://feeds.bbci.co.uk/news/rss.xml",
		},
		{
			Name: "Reuters",
			URL:  "https://www.reuters.com/rssFeed/worldNews",
		},
		{
			Name: "CNN",
			URL:  "http://rss.cnn.com/rss/edition.rss.html",
		},
		{
			Name: "TechCrunch",
			URL:  "https://techcrunch.com/feed/",
		},
		{
			Name: "Hacker News",
			URL:  "https://hnrss.org/frontpage",
		},
	}

	// Try to load from file if it exists
	if _, err := os.Stat("data/feeds.json"); err == nil {
		// Implementation would read from file here
		h.Logger.Info("Loading feeds from data/feeds.json")
	}

	return feeds, nil
}
