/*
Package handlers contains the core HTTP handlers for fetching, storing, and retrieving RSS feed data.

Key Functions:
  - HandleFetchAndStore: Fetches an RSS feed from a URL (POST) and stores the parsed items in Google Cloud Datastore.
  - HandleGetFeeds: Provides a list of predefined RSS feed sources.
  - HandleGetFeedItems: Retrieves RSS feed items with filtering options.

Usage:

	Import the package and register handlers in your router:
	  router.HandleFunc("/fetch-store", handlers.HandleFetchAndStore).Methods("POST")
	  router.HandleFunc("/feeds", handlers.HandleGetFeeds).Methods("GET")
	  router.HandleFunc("/items", handlers.HandleGetFeedItems).Methods("GET")
*/
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/Nexora-Open-Source/rss-feed-backend/middleware"
	"github.com/Nexora-Open-Source/rss-feed-backend/utils"

	"github.com/sirupsen/logrus"
)

// validateEnvironmentVariables validates required environment variables
func validateEnvironmentVariables() error {
	requiredVars := []string{"PROJECT_ID"}

	for _, varName := range requiredVars {
		if os.Getenv(varName) == "" {
			return fmt.Errorf("required environment variable %s is not set", varName)
		}
	}
	return nil
}

// validateAndSanitizeURL validates and sanitizes the input URL
func validateAndSanitizeURL(inputURL string) (string, error) {
	// Basic URL validation
	if inputURL == "" {
		return "", fmt.Errorf("URL cannot be empty")
	}

	// Parse the URL
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %v", err)
	}

	// Ensure the URL has a scheme
	if parsedURL.Scheme == "" {
		parsedURL.Scheme = "https"
	}

	// Only allow HTTP and HTTPS schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("only HTTP and HTTPS URLs are allowed")
	}

	// Validate the host
	if parsedURL.Host == "" {
		return "", fmt.Errorf("URL must have a valid host")
	}

	// Prevent localhost and private IP addresses (basic security)
	host := strings.ToLower(parsedURL.Host)
	if strings.Contains(host, "localhost") || strings.Contains(host, "127.0.0.1") {
		return "", fmt.Errorf("localhost URLs are not allowed")
	}

	// Return the sanitized URL
	return parsedURL.String(), nil
}

// FetchRequest represents the request body for POST /fetch-store
type FetchRequest struct {
	URL          string `json:"url" validate:"required"`
	Async        bool   `json:"async,omitempty"`
	ForceRefresh bool   `json:"force_refresh,omitempty"`
}

// FetchResponse represents the response for fetch operations
type FetchResponse struct {
	Success    bool        `json:"success"`
	Message    string      `json:"message"`
	Data       interface{} `json:"data,omitempty"`
	JobID      string      `json:"job_id,omitempty"`
	RequestID  string      `json:"request_id"`
	ItemsCount int         `json:"items_count,omitempty"`
	Source     string      `json:"source,omitempty"`
	Cache      string      `json:"cache,omitempty"`
	Status     string      `json:"status,omitempty"`
}

// @title RSS Feed Backend API
// @version 1.0
// @description A RESTful API for fetching, storing, and retrieving RSS feed data with filtering and pagination support.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /

// @Summary Fetch and store RSS feed
// @Description Fetches an RSS feed from a URL and stores the parsed items in Google Cloud Datastore. Supports both synchronous and asynchronous processing.
// @Tags RSS Feed Operations
// @Accept json
// @Produce json
// @Param request body FetchRequest true "RSS feed fetch request"
// @Success 200 {object} FetchResponse "Feed items fetched and stored successfully"
// @Success 202 {object} FetchResponse "Job submitted for async processing"
// @Failure 400 {object} middleware.APIError "Bad request"
// @Failure 500 {object} middleware.APIError "Internal server error"
// @Router /fetch-store [post]
func (h *Handler) HandleFetchAndStore(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = utils.GenerateRequestID()
		w.Header().Set("X-Request-ID", requestID)
	}

	// Parse request body
	var req FetchRequest
	if r.Body == nil {
		middleware.RespondBadRequest(w, fmt.Errorf("request body is required"), requestID)
		return
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.RespondBadRequest(w, fmt.Errorf("invalid request body: %v", err), requestID)
		return
	}

	// Validate required URL field
	if req.URL == "" {
		middleware.RespondBadRequest(w, fmt.Errorf("URL field is required"), requestID)
		return
	}

	// Validate and sanitize the URL
	sanitizedURL, err := validateAndSanitizeURL(req.URL)
	if err != nil {
		middleware.RespondValidationError(w, err, requestID)
		return
	}

	if req.Async {
		// Submit job for async processing
		jobID, err := h.AsyncProcessor.SubmitJob(sanitizedURL, requestID)
		if err != nil {
			middleware.Logger.WithFields(logrus.Fields{
				"request_id": requestID,
				"url":        sanitizedURL,
				"error":      err.Error(),
			}).Error("Failed to submit async job")
			middleware.RespondInternalError(w, err, requestID)
			return
		}

		response := FetchResponse{
			Success:   true,
			Message:   "Job submitted for async processing",
			JobID:     jobID,
			RequestID: requestID,
			Status:    "submitted",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Log the request
	middleware.Logger.WithFields(logrus.Fields{
		"request_id":    requestID,
		"url":           sanitizedURL,
		"action":        "fetch_and_store",
		"force_refresh": req.ForceRefresh,
	}).Info("Processing RSS feed request")

	// Sync processing - check cache first
	if !req.ForceRefresh {
		cachedItems, found := h.CacheManager.GetFeedItems(sanitizedURL)
		if found {
			middleware.Logger.WithFields(logrus.Fields{
				"request_id":  requestID,
				"url":         sanitizedURL,
				"items_count": len(cachedItems),
				"source":      "cache",
			}).Info("RSS feed retrieved from cache")

			response := FetchResponse{
				Success:    true,
				Message:    "RSS feed retrieved successfully",
				Data:       cachedItems,
				RequestID:  requestID,
				ItemsCount: len(cachedItems),
				Source:     "cache",
				Cache:      "HIT",
			}

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// Parse the RSS feed
	feedItems, err := utils.FetchRSSFeed(sanitizedURL)
	if err != nil {
		middleware.Logger.WithFields(logrus.Fields{
			"request_id": requestID,
			"url":        sanitizedURL,
			"error":      err.Error(),
		}).Error("Failed to fetch RSS feed")
		middleware.RespondExternalAPIError(w, err, requestID)
		return
	}

	// Save the feed items to Datastore
	if err := SaveToDatastore(h.DatastoreClient, feedItems); err != nil {
		middleware.Logger.WithFields(logrus.Fields{
			"request_id":  requestID,
			"url":         sanitizedURL,
			"items_count": len(feedItems),
			"error":       err.Error(),
		}).Error("Failed to save to Datastore")
		middleware.RespondInternalError(w, err, requestID)
		return
	}

	// Cache the results
	if err := h.CacheManager.SetFeedItems(sanitizedURL, feedItems); err != nil {
		middleware.Logger.WithFields(logrus.Fields{
			"request_id": requestID,
			"url":        sanitizedURL,
			"error":      err.Error(),
		}).Warn("Failed to cache RSS feed")
	}

	// Log successful completion
	middleware.Logger.WithFields(logrus.Fields{
		"request_id":  requestID,
		"url":         sanitizedURL,
		"items_count": len(feedItems),
		"source":      "live",
	}).Info("RSS feed processed successfully")

	response := FetchResponse{
		Success:    true,
		Message:    "RSS feed processed and stored successfully",
		Data:       feedItems,
		RequestID:  requestID,
		ItemsCount: len(feedItems),
		Source:     "live",
		Cache:      "MISS",
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
