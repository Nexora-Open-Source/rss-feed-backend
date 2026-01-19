/*
Package utils provides data management utilities for the RSS feed backend.

Key Functions:
  - OptimizeQueries: Provides query optimization recommendations
  - ValidateIndexes: Checks if required indexes exist
  - GetDataManagementConfig: Returns data management configuration

Usage:

	config := GetDataManagementConfig()
	optimized := OptimizeQueries(query, config)
*/
package utils

import (
	"fmt"
	"time"
)

// DataManagementConfig contains configuration for data management operations
type DataManagementConfig struct {
	// Validation settings
	Validation ValidationConfig `json:"validation"`

	// Duplicate detection settings
	DuplicateDetection DuplicateDetectionConfig `json:"duplicate_detection"`

	// Cleanup settings
	Cleanup CleanupConfig `json:"cleanup"`

	// Index optimization settings
	Indexes IndexConfig `json:"indexes"`
}

// ValidationConfig contains validation settings
type ValidationConfig struct {
	MaxTitleLength       int  `json:"max_title_length"`
	MaxDescriptionLength int  `json:"max_description_length"`
	MaxAuthorLength      int  `json:"max_author_length"`
	RequireTitle         bool `json:"require_title"`
	RequireLink          bool `json:"require_link"`
	ValidateURL          bool `json:"validate_url"`
	ValidateDate         bool `json:"validate_date"`
}

// DuplicateDetectionConfig contains duplicate detection settings
type DuplicateDetectionConfig struct {
	UseLinkComparison   bool   `json:"use_link_comparison"`
	UseContentHash      bool   `json:"use_content_hash"`
	UseTitleAuthorMatch bool   `json:"use_title_author_match"`
	HashAlgorithm       string `json:"hash_algorithm"`
	CaseSensitive       bool   `json:"case_sensitive"`
}

// CleanupConfig contains cleanup settings
type CleanupConfig struct {
	DefaultRetentionDays int  `json:"default_retention_days"`
	EnableAutoCleanup    bool `json:"enable_auto_cleanup"`
	CleanupBatchSize     int  `json:"cleanup_batch_size"`
	ScheduleCleanup      bool `json:"schedule_cleanup"`
	CleanupHour          int  `json:"cleanup_hour"` // Hour of day to run cleanup (0-23)
}

// IndexConfig contains index optimization settings
type IndexConfig struct {
	RequiredIndexes     []string `json:"required_indexes"`
	OptimizedQueries    []string `json:"optimized_queries"`
	EnableIndexHints    bool     `json:"enable_index_hints"`
	QueryTimeoutSeconds int      `json:"query_timeout_seconds"`
	MaxQueryResults     int      `json:"max_query_results"`
}

// GetDataManagementConfig returns the default data management configuration
func GetDataManagementConfig() DataManagementConfig {
	return DataManagementConfig{
		Validation: ValidationConfig{
			MaxTitleLength:       500,
			MaxDescriptionLength: 2000,
			MaxAuthorLength:      100,
			RequireTitle:         true,
			RequireLink:          true,
			ValidateURL:          true,
			ValidateDate:         true,
		},
		DuplicateDetection: DuplicateDetectionConfig{
			UseLinkComparison:   true,
			UseContentHash:      true,
			UseTitleAuthorMatch: true,
			HashAlgorithm:       "md5",
			CaseSensitive:       false,
		},
		Cleanup: CleanupConfig{
			DefaultRetentionDays: 30,
			EnableAutoCleanup:    true,
			CleanupBatchSize:     100,
			ScheduleCleanup:      true,
			CleanupHour:          2, // 2 AM
		},
		Indexes: IndexConfig{
			RequiredIndexes: []string{
				"pub_date_desc",
				"link_pub_date_desc",
				"author_pub_date_desc",
				"pub_date_range",
				"pub_date_asc",
				"link_only",
				"pub_date_key_pagination",
			},
			OptimizedQueries: []string{
				"fetch_items_by_date",
				"filter_by_source",
				"filter_by_author",
				"date_range_query",
				"cleanup_old_items",
			},
			EnableIndexHints:    true,
			QueryTimeoutSeconds: 30,
			MaxQueryResults:     1000,
		},
	}
}

// GetCleanupCutoffDate returns the cutoff date for cleanup based on retention days
func GetCleanupCutoffDate(retentionDays int) time.Time {
	return time.Now().AddDate(0, 0, -retentionDays)
}

// ValidateDataManagementConfig validates the data management configuration
func ValidateDataManagementConfig(config DataManagementConfig) error {
	// Validate retention days
	if config.Cleanup.DefaultRetentionDays < 1 || config.Cleanup.DefaultRetentionDays > 365 {
		return fmt.Errorf("cleanup retention days must be between 1 and 365")
	}

	// Validate cleanup hour
	if config.Cleanup.CleanupHour < 0 || config.Cleanup.CleanupHour > 23 {
		return fmt.Errorf("cleanup hour must be between 0 and 23")
	}

	// Validate batch sizes
	if config.Cleanup.CleanupBatchSize < 1 || config.Cleanup.CleanupBatchSize > 1000 {
		return fmt.Errorf("cleanup batch size must be between 1 and 1000")
	}

	// Validate query timeout
	if config.Indexes.QueryTimeoutSeconds < 5 || config.Indexes.QueryTimeoutSeconds > 300 {
		return fmt.Errorf("query timeout must be between 5 and 300 seconds")
	}

	// Validate max query results
	if config.Indexes.MaxQueryResults < 10 || config.Indexes.MaxQueryResults > 10000 {
		return fmt.Errorf("max query results must be between 10 and 10000")
	}

	return nil
}

// GetRecommendedIndexes returns a list of recommended indexes based on query patterns
func GetRecommendedIndexes() []string {
	return []string{
		// Primary query patterns
		"pub_date_desc",        // Most common: order by publication date
		"link_pub_date_desc",   // Source filtering with date ordering
		"author_pub_date_desc", // Author filtering with date ordering

		// Range queries
		"pub_date_range", // Date range queries
		"pub_date_asc",   // For cleanup operations

		// Lookup queries
		"link_only",   // Duplicate detection
		"author_only", // Author-based queries

		// Pagination optimization
		"pub_date_key_pagination", // Cursor-based pagination

		// Composite queries
		"link_date_range",   // Source + date range
		"author_date_range", // Author + date range
	}
}

// EstimateIndexUsage provides usage estimates for different index types
func EstimateIndexUsage() map[string]float64 {
	return map[string]float64{
		"pub_date_desc":           0.95, // Very high usage - primary ordering
		"link_pub_date_desc":      0.75, // High usage - source filtering
		"author_pub_date_desc":    0.60, // Medium-high usage - author filtering
		"pub_date_range":          0.80, // High usage - date filtering
		"link_only":               0.90, // Very high usage - duplicate detection
		"pub_date_asc":            0.40, // Medium usage - cleanup operations
		"pub_date_key_pagination": 0.70, // High usage - pagination
		"link_date_range":         0.50, // Medium usage - advanced filtering
		"author_date_range":       0.45, // Medium usage - advanced filtering
	}
}
