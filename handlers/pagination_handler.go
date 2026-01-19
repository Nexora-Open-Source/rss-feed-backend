/*
Package handlers provides HTTP handlers for paginated RSS feed item retrieval.

This package implements pagination functionality to handle large datasets efficiently.
*/
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Nexora-Open-Source/rss-feed-backend/middleware"
	"github.com/Nexora-Open-Source/rss-feed-backend/utils"
	"github.com/sirupsen/logrus"
)

// FilterParams represents filtering parameters for feed items
type FilterParams struct {
	Source   string `json:"source"`    // Filter by source URL/domain
	Author   string `json:"author"`    // Filter by author
	DateFrom string `json:"date_from"` // Filter by date from (RFC3339 format)
	DateTo   string `json:"date_to"`   // Filter by date to (RFC3339 format)
	Keyword  string `json:"keyword"`   // Filter by keyword in title or description
}

// ItemsQueryParams represents all query parameters for items endpoint
type ItemsQueryParams struct {
	PaginationParams
	FilterParams
}

// @Summary Get RSS feed items with filtering
// @Description Retrieves RSS feed items from Google Cloud Datastore with pagination and filtering support.
// @Tags RSS Feed Operations
// @Accept json
// @Produce json
// @Param limit query int false "Number of items to return (default: 100, max: 1000)"
// @Param offset query int false "Number of items to skip (default: 0)"
// @Param cursor query string false "Pagination cursor for cursor-based pagination"
// @Param source query string false "Filter by source URL/domain"
// @Param author query string false "Filter by author"
// @Param date_from query string false "Filter by date from (RFC3339 format)"
// @Param date_to query string false "Filter by date to (RFC3339 format)"
// @Param keyword query string false "Filter by keyword in title or description"
// @Success 200 {object} PaginatedResult "Feed items retrieved successfully"
// @Failure 400 {object} middleware.APIError "Bad request"
// @Failure 500 {object} middleware.APIError "Internal server error"
// @Router /items [get]
func (h *Handler) HandleGetFeedItems(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = utils.GenerateRequestID()
		w.Header().Set("X-Request-ID", requestID)
	}

	// Parse pagination parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	cursor := r.URL.Query().Get("cursor")

	limit := 100 // default limit
	offset := 0  // default offset

	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			limit = parsedLimit
		} else {
			middleware.RespondBadRequest(w, fmt.Errorf("invalid limit parameter: %v", err), requestID)
			return
		}
	}

	if offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsedOffset
		} else {
			middleware.RespondBadRequest(w, fmt.Errorf("invalid offset parameter: %v", err), requestID)
			return
		}
	}

	// Handle cursor-based pagination
	if cursor != "" {
		// Simple cursor parsing (in a real implementation, you might use more sophisticated cursor encoding)
		if len(cursor) > 7 && cursor[:7] == "offset:" {
			if cursorOffset, err := strconv.Atoi(cursor[7:]); err == nil {
				offset = cursorOffset
			}
		}
	}

	// Parse filter parameters
	filterParams := FilterParams{
		Source:   r.URL.Query().Get("source"),
		Author:   r.URL.Query().Get("author"),
		DateFrom: r.URL.Query().Get("date_from"),
		DateTo:   r.URL.Query().Get("date_to"),
		Keyword:  r.URL.Query().Get("keyword"),
	}

	// Validate date parameters
	if filterParams.DateFrom != "" {
		if _, err := time.Parse(time.RFC3339, filterParams.DateFrom); err != nil {
			middleware.RespondBadRequest(w, fmt.Errorf("invalid date_from parameter, expected RFC3339 format: %v", err), requestID)
			return
		}
	}

	if filterParams.DateTo != "" {
		if _, err := time.Parse(time.RFC3339, filterParams.DateTo); err != nil {
			middleware.RespondBadRequest(w, fmt.Errorf("invalid date_to parameter, expected RFC3339 format: %v", err), requestID)
			return
		}
	}

	// Create query parameters
	params := ItemsQueryParams{
		PaginationParams: PaginationParams{
			Limit:  limit,
			Offset: offset,
			Cursor: cursor,
		},
		FilterParams: filterParams,
	}

	// Log the request
	middleware.Logger.WithFields(logrus.Fields{
		"request_id": requestID,
		"action":     "get_feed_items",
		"limit":      limit,
		"offset":     offset,
		"cursor":     cursor,
		"source":     filterParams.Source,
		"author":     filterParams.Author,
		"date_from":  filterParams.DateFrom,
		"date_to":    filterParams.DateTo,
		"keyword":    filterParams.Keyword,
	}).Info("Processing filtered feed items request")

	// Check cache first
	cacheKey := fmt.Sprintf("items:limit:%d:offset:%d:cursor:%s:source:%s:author:%s:date_from:%s:date_to:%s:keyword:%s",
		limit, offset, cursor, filterParams.Source, filterParams.Author, filterParams.DateFrom, filterParams.DateTo, filterParams.Keyword)
	cachedResult, found := h.CacheManager.GetStoredItems(cacheKey)
	if found {
		// Convert cached items to paginated result
		result := &PaginatedResult{
			Items:      cachedResult,
			TotalCount: len(cachedResult), // Note: This is simplified
			HasMore:    len(cachedResult) == limit,
		}

		middleware.Logger.WithFields(logrus.Fields{
			"request_id":  requestID,
			"items_count": len(cachedResult),
			"source":      "cache",
		}).Info("Feed items retrieved from cache")

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(result)
		return
	}

	// Fetch items from datastore with filtering
	result, err := FetchFeedItemsWithFilter(h.DatastoreClient, params)
	if err != nil {
		middleware.Logger.WithFields(logrus.Fields{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to fetch feed items")
		middleware.RespondInternalError(w, err, requestID)
		return
	}

	// Cache the result
	if err := h.CacheManager.SetStoredItems(cacheKey, result.Items); err != nil {
		middleware.Logger.WithFields(logrus.Fields{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Failed to cache feed items")
	}

	// Log successful completion
	middleware.Logger.WithFields(logrus.Fields{
		"request_id":  requestID,
		"items_count": len(result.Items),
		"total_count": result.TotalCount,
		"has_more":    result.HasMore,
		"source":      "datastore",
	}).Info("Feed items retrieved successfully")

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

/*
HandleGetFeedItemsLegacy retrieves all RSS feed items (legacy endpoint for backward compatibility).

Example:

	GET /items/legacy

Response:
  - 200 OK: Array of feed items.
  - 500 Internal Server Error: Failed to fetch items.
*/
func (h *Handler) HandleGetFeedItemsLegacy(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = utils.GenerateRequestID()
		w.Header().Set("X-Request-ID", requestID)
	}

	// Log the request
	middleware.Logger.WithFields(logrus.Fields{
		"request_id": requestID,
		"action":     "get_feed_items_legacy",
	}).Info("Processing legacy feed items request")

	// Fetch items using legacy function
	items, err := FetchFeedItemsLegacy(h.DatastoreClient)
	if err != nil {
		middleware.Logger.WithFields(logrus.Fields{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to fetch legacy feed items")
		middleware.RespondInternalError(w, err, requestID)
		return
	}

	// Log successful completion
	middleware.Logger.WithFields(logrus.Fields{
		"request_id":  requestID,
		"items_count": len(items),
	}).Info("Legacy feed items retrieved successfully")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(items)
}
