// Package rss provides RSS feed processing handlers for the RSS feed backend
package rss

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"cloud.google.com/go/datastore"
	"github.com/Nexora-Open-Source/rss-feed-backend/cache"
	"github.com/Nexora-Open-Source/rss-feed-backend/middleware"
	"github.com/Nexora-Open-Source/rss-feed-backend/types"
	"github.com/Nexora-Open-Source/rss-feed-backend/utils"
	"github.com/sirupsen/logrus"
)

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

// AsyncProcessorInterface defines the interface for async processor operations
type AsyncProcessorInterface interface {
	SubmitJob(url, requestID string) (string, error)
	GetJobStatus(jobID string) (*types.AsyncJobStatus, bool)
}

// Handler contains dependencies for RSS handlers
type Handler struct {
	DatastoreClient *datastore.Client
	CacheManager    *cache.CacheManager
	Logger          *logrus.Logger
	AsyncProcessor  AsyncProcessorInterface
}

// NewHandler creates a new RSS handler
func NewHandler(datastoreClient *datastore.Client, cacheManager *cache.CacheManager, logger *logrus.Logger, asyncProcessor AsyncProcessorInterface) *Handler {
	return &Handler{
		DatastoreClient: datastoreClient,
		CacheManager:    cacheManager,
		Logger:          logger,
		AsyncProcessor:  asyncProcessor,
	}
}

// HandleFetchAndStore fetches and stores RSS feed data
func (h *Handler) HandleFetchAndStore(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = utils.GenerateRequestID()
		w.Header().Set("X-Request-ID", requestID)
	}

	// Parse request body
	var req FetchRequest
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
	sanitizedURL, err := h.validateAndSanitizeURL(req.URL)
	if err != nil {
		middleware.RespondValidationError(w, err, requestID)
		return
	}

	if req.Async {
		// Submit job for async processing
		jobID, err := h.AsyncProcessor.SubmitJob(sanitizedURL, requestID)
		if err != nil {
			h.Logger.WithFields(logrus.Fields{
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
	h.Logger.WithFields(logrus.Fields{
		"request_id":    requestID,
		"url":           sanitizedURL,
		"action":        "fetch_and_store",
		"force_refresh": req.ForceRefresh,
	}).Info("Processing RSS feed request")

	// Sync processing - check cache first
	if !req.ForceRefresh {
		cachedItems, found := h.CacheManager.GetFeedItems(sanitizedURL)
		if found {
			h.Logger.WithFields(logrus.Fields{
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
		h.Logger.WithFields(logrus.Fields{
			"request_id": requestID,
			"url":        sanitizedURL,
			"error":      err.Error(),
		}).Error("Failed to fetch RSS feed")
		middleware.RespondExternalAPIError(w, err, requestID)
		return
	}

	// Save the feed items to Datastore
	if err := h.saveToDatastore(feedItems); err != nil {
		h.Logger.WithFields(logrus.Fields{
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
		h.Logger.WithFields(logrus.Fields{
			"request_id": requestID,
			"url":        sanitizedURL,
			"error":      err.Error(),
		}).Warn("Failed to cache RSS feed")
	}

	// Log successful completion
	h.Logger.WithFields(logrus.Fields{
		"request_id":  requestID,
		"url":         sanitizedURL,
		"items_count": len(feedItems),
		"source":      "datastore",
	}).Info("RSS feed processed successfully")

	response := FetchResponse{
		Success:    true,
		Message:    "RSS feed fetched and stored successfully",
		Data:       feedItems,
		RequestID:  requestID,
		ItemsCount: len(feedItems),
		Source:     "datastore",
		Cache:      "MISS",
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// validateAndSanitizeURL validates and sanitizes the input URL with enhanced security checks
func (h *Handler) validateAndSanitizeURL(inputURL string) (string, error) {
	// Basic URL validation
	if inputURL == "" {
		return "", fmt.Errorf("URL cannot be empty")
	}

	// Check for excessive length (prevent DoS)
	if len(inputURL) > 2048 {
		return "", fmt.Errorf("URL length exceeds maximum allowed size")
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

	// Enhanced security checks
	host := strings.ToLower(parsedURL.Host)

	// Block localhost and private IP ranges
	if h.isPrivateOrLocalhost(host) {
		return "", fmt.Errorf("access to private networks and localhost is not allowed")
	}

	// Block suspicious file extensions in path
	if h.hasSuspiciousFileExtension(parsedURL.Path) {
		return "", fmt.Errorf("URL contains suspicious file extension")
	}

	// Validate against common RSS feed patterns
	if !h.isValidRSSURL(parsedURL) {
		h.Logger.WithField("url", parsedURL.String()).Warn("URL does not match typical RSS feed patterns")
		// Not blocking, just warning since RSS feeds can have various formats
	}

	// Additional security: Check for script injection in query parameters
	if h.hasScriptInjection(parsedURL.Query()) {
		return "", fmt.Errorf("URL contains potentially malicious content")
	}

	// Return the sanitized URL
	return parsedURL.String(), nil
}

// isPrivateOrLocalhost checks if the host is a private IP or localhost
func (h *Handler) isPrivateOrLocalhost(host string) bool {
	// Remove port if present
	hostOnly := host
	if strings.Contains(host, ":") {
		hostOnly = strings.Split(host, ":")[0]
	}

	// Check for localhost variations
	localhostPatterns := []string{
		"localhost", "127.0.0.1", "::1", "0.0.0.0",
		"0:0:0:0:0:0:0:1", "0:0:0:0:0:0:0:0",
	}
	for _, pattern := range localhostPatterns {
		if hostOnly == pattern {
			return true
		}
	}

	// Try to parse as IP address
	ip := net.ParseIP(hostOnly)
	if ip != nil {
		// Check if it's a private IP
		return ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified()
	}

	// Check for private domain patterns
	privateDomainPatterns := []string{
		".local", ".localhost", ".internal", ".corp", ".home",
		".lan", ".priv", ".test", ".dev",
	}
	for _, pattern := range privateDomainPatterns {
		if strings.HasSuffix(hostOnly, pattern) {
			return true
		}
	}

	return false
}

// hasSuspiciousFileExtension checks for potentially dangerous file extensions
func (h *Handler) hasSuspiciousFileExtension(path string) bool {
	suspiciousExtensions := []string{
		".exe", ".bat", ".cmd", ".com", ".pif", ".scr", ".vbs", ".js", ".jar",
		".php", ".asp", ".aspx", ".jsp", ".cgi", ".pl", ".py", ".rb", ".sh",
		".ps1", ".psm1", ".psd1", ".bat", ".cmd", ".wsf", ".wsh",
	}

	lowerPath := strings.ToLower(path)
	for _, ext := range suspiciousExtensions {
		if strings.Contains(lowerPath, ext) {
			return true
		}
	}
	return false
}

// isValidRSSURL checks if the URL looks like a valid RSS feed URL
func (h *Handler) isValidRSSURL(parsedURL *url.URL) bool {
	// Common RSS feed path patterns
	rssPatterns := []string{
		`(?i)/rss`,
		`(?i)/feed`,
		`(?i)/atom`,
		`(?i)/xml`,
		`(?i)\.rss$`,
		`(?i)\.xml$`,
		`(?i)\.atom$`,
	}

	path := strings.ToLower(parsedURL.Path)
	for _, pattern := range rssPatterns {
		matched, _ := regexp.MatchString(pattern, path)
		if matched {
			return true
		}
	}

	// Check if query parameters suggest RSS feed
	query := strings.ToLower(parsedURL.RawQuery)
	if strings.Contains(query, "rss") || strings.Contains(query, "feed") || strings.Contains(query, "atom") {
		return true
	}

	return false
}

// hasScriptInjection checks for potential script injection in query parameters
func (h *Handler) hasScriptInjection(query url.Values) bool {
	scriptPatterns := []string{
		`<script`, `javascript:`, `vbscript:`, `onload=`, `onerror=`,
		`eval\(`, `alert\(`, `prompt\(`, `confirm\(`,
		`document\.`, `window\.`, `location\.`, `cookie`,
	}

	for _, values := range query {
		for _, value := range values {
			lowerValue := strings.ToLower(value)
			for _, pattern := range scriptPatterns {
				matched, _ := regexp.MatchString(pattern, lowerValue)
				if matched {
					return true
				}
			}
		}
	}

	return false
}

// saveToDatastore saves feed items to Datastore
func (h *Handler) saveToDatastore(items []*utils.FeedItem) error {
	ctx := context.Background()
	batchSize := 500

	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}

		batch := items[i:end]
		keys := make([]*datastore.Key, len(batch))

		// Prepare keys for the batch
		for j, item := range batch {
			// Use the Link as the unique key to prevent duplicates
			keys[j] = datastore.NameKey("FeedItem", item.Link, nil)
		}

		// Perform batch put operation
		_, err := h.DatastoreClient.PutMulti(ctx, keys, batch)
		if err != nil {
			return fmt.Errorf("batch save failed at batch starting index %d: %v", i, err)
		}
	}

	return nil
}
