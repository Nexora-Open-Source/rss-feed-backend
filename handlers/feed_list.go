package handlers

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

// @Summary Get predefined RSS feed sources
// @Description Returns a list of predefined RSS feed sources from a JSON file.
// @Tags RSS Feed Operations
// @Accept json
// @Produce json
// @Success 200 {array} FeedSource "List of predefined feed sources"
// @Failure 500 {object} middleware.APIError "Internal server error"
// @Router /feeds [get]
func (h *Handler) HandleGetFeeds(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = utils.GenerateRequestID()
		w.Header().Set("X-Request-ID", requestID)
	}

	// Log the request
	middleware.Logger.WithFields(logrus.Fields{
		"request_id": requestID,
		"action":     "get_feeds",
	}).Info("Processing feed list request")

	// Define the path to the JSON file
	filePath := "data/feeds.json"

	// Try to get the working directory if the file doesn't exist
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Try alternative paths for test environment
		altPaths := []string{
			"../data/feeds.json",
			"../../data/feeds.json",
			"../../../data/feeds.json",
		}
		for _, altPath := range altPaths {
			if _, err := os.Stat(altPath); err == nil {
				filePath = altPath
				break
			}
		}
	}

	// Open the JSON file
	file, err := os.Open(filePath)
	if err != nil {
		middleware.Logger.WithFields(logrus.Fields{
			"request_id": requestID,
			"file_path":  filePath,
			"error":      err.Error(),
		}).Error("Error opening feeds.json file, using fallback feeds")

		// Fallback to hardcoded feeds if file is not found
		feeds := []FeedSource{
			{Name: "TechCrunch", URL: "https://techcrunch.com/feed/"},
			{Name: "BBC News", URL: "http://feeds.bbci.co.uk/news/rss.xml"},
			{Name: "The Verge", URL: "https://www.theverge.com/rss/index.xml"},
			{Name: "CNN Top Stories", URL: "http://rss.cnn.com/rss/edition.rss"},
			{Name: "Hacker News", URL: "https://hnrss.org/frontpage"},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(feeds)
		return
	}
	defer file.Close()

	// Decode the JSON data
	var feeds []FeedSource
	if err := json.NewDecoder(file).Decode(&feeds); err != nil {
		middleware.Logger.WithFields(logrus.Fields{
			"request_id": requestID,
			"file_path":  filePath,
			"error":      err.Error(),
		}).Error("Error decoding feeds.json file")
		middleware.RespondInternalError(w, err, requestID)
		return
	}

	// Log successful completion
	middleware.Logger.WithFields(logrus.Fields{
		"request_id":  requestID,
		"feeds_count": len(feeds),
	}).Info("Feed list retrieved successfully")

	// Respond with the list of feeds
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(feeds)
}
