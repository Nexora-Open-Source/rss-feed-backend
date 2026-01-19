/*
Package handlers provides functions to interact with Google Cloud Datastore.

Key Functions:
  - SaveToDatastore: Stores RSS feed items in Datastore with batch operations.
  - FetchFeedItems: Retrieves stored feed items from Datastore with pagination.
  - BatchSaveToDatastore: Performs batch save operations for better performance.
*/
package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Nexora-Open-Source/rss-feed-backend/utils"

	"cloud.google.com/go/datastore"
)

/*
SaveToDatastore saves a list of RSS feed items to Google Cloud Datastore using batch operations.

Parameters:
  - client: Datastore client instance
  - items: A slice of FeedItem objects to store.
  - batchSize: The number of items to process in each batch (optional, will use adaptive sizing if 0).

Errors:

	Returns an error if Datastore operation fails.

Usage:

	client := datastore.NewClient(...)
	items := []*utils.FeedItem{...}
	err := SaveToDatastore(client, items, 0) // Use adaptive batch size
	if err != nil {
	    log.Fatalf("Failed to save feed items: %v", err)
	}
*/
func SaveToDatastore(client DatastoreClientInterface, items []*utils.FeedItem, batchSize ...int) error {
	adaptiveBatchSize := calculateAdaptiveBatchSize(len(items), getBatchSizeFromConfig(batchSize...))
	_, err := BatchSaveToDatastoreWithDeduplication(client, items, adaptiveBatchSize)
	return err
}

/*
BatchSaveToDatastore saves RSS feed items using batch operations for better performance.

Parameters:
  - client: Datastore client instance
  - items: A slice of FeedItem objects to store.
  - batchSize: The number of items to process in each batch.

Errors:

	Returns an error if any Datastore operation fails.

Usage:

	client := datastore.NewClient(...)
	items := []*utils.FeedItem{...}
	err := BatchSaveToDatastore(client, items, 1000)
	if err != nil {
	    log.Fatalf("Failed to save feed items: %v", err)
	}
*/
func BatchSaveToDatastore(client DatastoreClientInterface, items []*utils.FeedItem, batchSize int) error {
	ctx := context.Background()

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
		_, err := client.PutMulti(ctx, keys, batch)
		if err != nil {
			return fmt.Errorf("batch save failed at batch starting index %d: %v", i, err)
		}
	}

	return nil
}

// calculateAdaptiveBatchSize determines optimal batch size based on feed characteristics
func calculateAdaptiveBatchSize(itemCount int, configuredBatchSize int) int {
	if configuredBatchSize > 0 {
		return configuredBatchSize
	}

	// Adaptive batch sizing based on item count
	switch {
	case itemCount <= 10:
		return 50 // Small batch for small feeds
	case itemCount <= 50:
		return 200 // Medium batch for medium feeds
	case itemCount <= 200:
		return 500 // Large batch for larger feeds
	case itemCount <= 1000:
		return 1000 // Extra large batch for very large feeds
	default:
		return 2000 // Maximum batch size for huge feeds
	}
}

// getBatchSizeFromConfig extracts batch size from variadic parameters
func getBatchSizeFromConfig(batchSize ...int) int {
	if len(batchSize) > 0 && batchSize[0] > 0 {
		return batchSize[0]
	}
	return 0 // Use adaptive sizing
}

/*
CheckForDuplicates checks if any items in the batch already exist in the datastore
using multiple duplicate detection strategies.

Parameters:
  - client: Datastore client instance
  - items: A slice of FeedItem objects to check for duplicates.

Returns:
  - A map of content hashes to existing items
  - An error if Datastore operation fails.
*/
func CheckForDuplicates(client DatastoreReaderInterface, items []*utils.FeedItem) (map[string]*utils.FeedItem, error) {
	ctx := context.Background()
	existingItems := make(map[string]*utils.FeedItem)

	// Collect all content hashes from new items
	hashes := make([]string, 0, len(items))
	for _, item := range items {
		hashes = append(hashes, item.GenerateContentHash())
	}

	// Query existing items by their links (primary duplicate detection)
	for _, item := range items {
		key := datastore.NameKey("FeedItem", item.Link, nil)
		var existing utils.FeedItem
		err := client.Get(ctx, key, &existing)
		if err == nil {
			// Item found by link
			existingItems[item.GenerateContentHash()] = &existing
		} else if err != datastore.ErrNoSuchEntity {
			return nil, fmt.Errorf("error checking for duplicates: %v", err)
		}
	}

	return existingItems, nil
}

/*
BatchSaveToDatastoreWithDeduplication saves RSS feed items using batch operations with duplicate detection.

Parameters:
  - client: Datastore client instance
  - items: A slice of FeedItem objects to store.
  - batchSize: The number of items to process in each batch.

Returns:
  - The number of new items saved (excluding duplicates)
  - An error if any Datastore operation fails.

Usage:

	client := datastore.NewClient(...)
	items := []*utils.FeedItem{...}
	newCount, err := BatchSaveToDatastoreWithDeduplication(client, items, 1000)
	if err != nil {
	    log.Fatalf("Failed to save feed items: %v", err)
	}
*/
func BatchSaveToDatastoreWithDeduplication(client DatastoreClientInterface, items []*utils.FeedItem, batchSize int) (int, error) {
	ctx := context.Background()
	newItemsCount := 0

	// Check for duplicates first
	existingItems, err := CheckForDuplicates(client, items)
	if err != nil {
		return 0, err
	}

	var uniqueItems []*utils.FeedItem
	for _, item := range items {
		itemHash := item.GenerateContentHash()
		if existing, exists := existingItems[itemHash]; exists {
			// Check if this is really a duplicate using multiple criteria
			if item.IsDuplicate(existing) {
				continue // Skip duplicate
			}
		}
		uniqueItems = append(uniqueItems, item)
	}

	// Save unique items in batches
	for i := 0; i < len(uniqueItems); i += batchSize {
		end := i + batchSize
		if end > len(uniqueItems) {
			end = len(uniqueItems)
		}

		batch := uniqueItems[i:end]
		keys := make([]*datastore.Key, len(batch))

		// Prepare keys for the batch
		for j, item := range batch {
			// Use the Link as the unique key to prevent duplicates
			keys[j] = datastore.NameKey("FeedItem", item.Link, nil)
		}

		// Perform batch put operation
		_, err := client.PutMulti(ctx, keys, batch)
		if err != nil {
			return newItemsCount, fmt.Errorf("batch save failed at batch starting index %d: %v", i, err)
		}

		newItemsCount += len(batch)
	}

	return newItemsCount, nil
}

/*
CleanupOldFeedItems removes feed items older than the specified date.

Parameters:
  - client: Datastore client instance
  - olderThan: Items older than this date will be deleted
  - batchSize: The number of items to delete in each batch

Returns:
  - The number of items deleted
  - An error if Datastore operation fails.

Usage:

	client := datastore.NewClient(...)
	cutoffDate := time.Now().AddDate(0, 0, -30) // 30 days ago
	deletedCount, err := CleanupOldFeedItems(client, cutoffDate, 100)
	if err != nil {
	    log.Fatalf("Failed to cleanup old feed items: %v", err)
	}
*/
func CleanupOldFeedItems(client DatastoreClientInterface, olderThan time.Time, batchSize int) (int, error) {
	ctx := context.Background()
	deletedCount := 0

	// Query for items older than the cutoff date
	query := datastore.NewQuery("FeedItem").
		Filter("pub_date <", olderThan.Format(time.RFC3339)).
		KeysOnly()

	// Get all keys for old items
	keys, err := client.GetAll(ctx, query, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to query old items: %v", err)
	}

	if len(keys) == 0 {
		return 0, nil // No items to delete
	}

	// Delete items in batches
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}

		batch := keys[i:end]
		err := client.DeleteMulti(ctx, batch)
		if err != nil {
			return deletedCount, fmt.Errorf("batch delete failed at batch starting index %d: %v", i, err)
		}

		deletedCount += len(batch)
	}

	return deletedCount, nil
}

/*
GetFeedItemStats returns statistics about the feed items in the datastore.

Parameters:
  - client: Datastore client instance

Returns:
  - Total number of items
  - Number of items by age ranges
  - An error if Datastore operation fails.

Usage:

	client := datastore.NewClient(...)
	total, stats, err := GetFeedItemStats(client)
	if err != nil {
	    log.Fatalf("Failed to get feed item stats: %v", err)
	}
*/
func GetFeedItemStats(client DatastoreClientInterface) (int, map[string]int, error) {
	ctx := context.Background()

	// Get total count
	totalQuery := datastore.NewQuery("FeedItem").KeysOnly()
	totalKeys, err := client.GetAll(ctx, totalQuery, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get total count: %v", err)
	}
	totalCount := len(totalKeys)

	// Get stats by age ranges
	now := time.Now()
	stats := map[string]int{
		"last_24h":  0,
		"last_7d":   0,
		"last_30d":  0,
		"older_30d": 0,
	}

	// Query items with date ranges
	timeRanges := []struct {
		name  string
		start time.Time
		end   time.Time
	}{
		{"last_24h", now.Add(-24 * time.Hour), now},
		{"last_7d", now.AddDate(0, 0, -7), now.Add(-24 * time.Hour)},
		{"last_30d", now.AddDate(0, 0, -30), now.AddDate(0, 0, -7)},
		{"older_30d", time.Time{}, now.AddDate(0, 0, -30)},
	}

	for _, tr := range timeRanges {
		var query *datastore.Query
		if tr.start.IsZero() {
			// For older_30d, just filter by end date
			query = datastore.NewQuery("FeedItem").
				Filter("pub_date <", tr.end.Format(time.RFC3339)).
				KeysOnly()
		} else {
			// For other ranges, filter between start and end
			query = datastore.NewQuery("FeedItem").
				Filter("pub_date >=", tr.start.Format(time.RFC3339)).
				Filter("pub_date <", tr.end.Format(time.RFC3339)).
				KeysOnly()
		}

		keys, err := client.GetAll(ctx, query, nil)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to get stats for %s: %v", tr.name, err)
		}
		stats[tr.name] = len(keys)
	}

	return totalCount, stats, nil
}

type PaginationParams struct {
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
	Cursor string `json:"cursor"`
}

// PaginatedResult represents a paginated result
type PaginatedResult struct {
	Items      []*utils.FeedItem `json:"items"`
	TotalCount int               `json:"total_count"`
	HasMore    bool              `json:"has_more"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

/*
FetchFeedItems retrieves RSS feed items from Google Cloud Datastore with pagination support.

Parameters:
  - client: Datastore client instance
  - params: Pagination parameters (limit, offset, cursor).

Returns:
  - A PaginatedResult containing feed items and pagination metadata.
  - An error if Datastore operation fails.

Usage:

	client := datastore.NewClient(...)
	params := PaginationParams{Limit: 50, Offset: 0}
	result, err := FetchFeedItems(client, params)
	if err != nil {
	    log.Fatalf("Failed to fetch feed items: %v", err)
	}
*/
func FetchFeedItems(client DatastoreReaderInterface, params PaginationParams) (*PaginatedResult, error) {
	ctx := context.Background()
	query := datastore.NewQuery("FeedItem")

	// Set default limit if not specified
	if params.Limit <= 0 {
		params.Limit = 100
	}
	if params.Limit > 1000 {
		params.Limit = 1000 // Maximum limit to prevent excessive resource usage
	}

	// Apply pagination
	query = query.Limit(params.Limit)
	if params.Offset > 0 {
		query = query.Offset(params.Offset)
	}

	// Order by publication date for consistent pagination
	query = query.Order("-pub_date")

	var items []*utils.FeedItem
	keys, err := client.GetAll(ctx, query, &items)
	if err != nil {
		return nil, err
	}

	// Get total count for pagination metadata
	countQuery := datastore.NewQuery("FeedItem").KeysOnly()
	totalKeys, err := client.GetAll(ctx, countQuery, nil)
	if err != nil {
		return nil, err
	}
	totalCount := len(totalKeys)

	// Generate next cursor if there are more items
	nextCursor := ""
	hasMore := (params.Offset + len(items)) < totalCount
	if hasMore && len(keys) > 0 {
		// Use cursor-based pagination for better performance
		// Note: In a real implementation, you might want to use cursor-based pagination
		// For now, we'll use offset-based pagination
		nextOffset := params.Offset + len(items)
		if nextOffset < totalCount {
			nextCursor = fmt.Sprintf("offset:%d", nextOffset)
		}
	}

	return &PaginatedResult{
		Items:      items,
		TotalCount: totalCount,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

/*
FetchFeedItemsWithFilter retrieves RSS feed items from Google Cloud Datastore with pagination and filtering support.

Parameters:
  - client: Datastore client instance
  - params: ItemsQueryParams containing pagination and filter parameters.

Returns:
  - A PaginatedResult containing filtered feed items and pagination metadata.
  - An error if Datastore operation fails.

Usage:

	client := datastore.NewClient(...)
	params := ItemsQueryParams{
		PaginationParams: PaginationParams{Limit: 50, Offset: 0},
		FilterParams: FilterParams{Source: "example.com", DateFrom: "2023-01-01T00:00:00Z"},
	}
	result, err := FetchFeedItemsWithFilter(client, params)
	if err != nil {
	    log.Fatalf("Failed to fetch filtered feed items: %v", err)
	}
*/
func FetchFeedItemsWithFilter(client DatastoreReaderInterface, params ItemsQueryParams) (*PaginatedResult, error) {
	ctx := context.Background()
	query := datastore.NewQuery("FeedItem")

	// Apply filters
	if params.Source != "" {
		// Filter by link containing the source
		query = query.Filter("link >", params.Source).Filter("link <", params.Source+"\ufffd")
	}

	if params.Author != "" {
		query = query.Filter("author =", params.Author)
	}

	// Apply date filters if provided
	if params.DateFrom != "" {
		if dateFrom, err := time.Parse(time.RFC3339, params.DateFrom); err == nil {
			query = query.Filter("pub_date >=", dateFrom.Format(time.RFC3339))
		}
	}

	if params.DateTo != "" {
		if dateTo, err := time.Parse(time.RFC3339, params.DateTo); err == nil {
			query = query.Filter("pub_date <=", dateTo.Format(time.RFC3339))
		}
	}

	// Set default limit if not specified
	if params.Limit <= 0 {
		params.Limit = 100
	}
	if params.Limit > 1000 {
		params.Limit = 1000 // Maximum limit to prevent excessive resource usage
	}

	// Apply pagination
	query = query.Limit(params.Limit)
	if params.Offset > 0 {
		query = query.Offset(params.Offset)
	}

	// Order by publication date for consistent pagination
	query = query.Order("-pub_date")

	var items []*utils.FeedItem
	keys, err := client.GetAll(ctx, query, &items)
	if err != nil {
		return nil, err
	}

	// Apply keyword filter (client-side filtering since Datastore doesn't support full-text search)
	if params.Keyword != "" {
		var filteredItems []*utils.FeedItem
		keyword := strings.ToLower(params.Keyword)

		for _, item := range items {
			if strings.Contains(strings.ToLower(item.Title), keyword) ||
				strings.Contains(strings.ToLower(item.Description), keyword) {
				filteredItems = append(filteredItems, item)
			}
		}
		items = filteredItems
	}

	// Get total count for pagination metadata (simplified - in production you'd want a more efficient count query)
	countQuery := datastore.NewQuery("FeedItem").KeysOnly()

	// Apply same filters to count query
	if params.Source != "" {
		countQuery = countQuery.Filter("link >", params.Source).Filter("link <", params.Source+"\ufffd")
	}
	if params.Author != "" {
		countQuery = countQuery.Filter("author =", params.Author)
	}
	if params.DateFrom != "" {
		if dateFrom, err := time.Parse(time.RFC3339, params.DateFrom); err == nil {
			countQuery = countQuery.Filter("pub_date >=", dateFrom.Format(time.RFC3339))
		}
	}
	if params.DateTo != "" {
		if dateTo, err := time.Parse(time.RFC3339, params.DateTo); err == nil {
			countQuery = countQuery.Filter("pub_date <=", dateTo.Format(time.RFC3339))
		}
	}

	totalKeys, err := client.GetAll(ctx, countQuery, nil)
	if err != nil {
		return nil, err
	}
	totalCount := len(totalKeys)

	// Generate next cursor if there are more items
	nextCursor := ""
	hasMore := (params.Offset + len(items)) < totalCount
	if hasMore && len(keys) > 0 {
		nextOffset := params.Offset + len(items)
		if nextOffset < totalCount {
			nextCursor = fmt.Sprintf("offset:%d", nextOffset)
		}
	}

	return &PaginatedResult{
		Items:      items,
		TotalCount: totalCount,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

/*
FetchFeedItemsLegacy retrieves all RSS feed items stored in Google Cloud Datastore (legacy function).

Parameters:
  - client: Datastore client instance

Returns:
  - A slice of FeedItem objects.
  - An error if Datastore operation fails.

Usage:

	client := datastore.NewClient(...)
	items, err := FetchFeedItemsLegacy(client)
	if err != nil {
	    log.Fatalf("Failed to fetch feed items: %v", err)
	}
*/
func FetchFeedItemsLegacy(client DatastoreReaderInterface) ([]*utils.FeedItem, error) {
	params := PaginationParams{Limit: 1000} // Use reasonable limit
	result, err := FetchFeedItems(client, params)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}
