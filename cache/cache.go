/*
Package cache provides caching functionality for RSS feeds.

This package implements both in-memory and Redis-based caching to improve
performance by reducing redundant RSS feed fetching and Datastore operations.
*/
package cache

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/Nexora-Open-Source/rss-feed-backend/utils"
	"github.com/sirupsen/logrus"
)

// CacheItem represents a cached item with expiration
type CacheItem struct {
	Data      []*utils.FeedItem `json:"data"`
	ExpiresAt time.Time         `json:"expires_at"`
}

// IsExpired checks if the cache item has expired
func (c *CacheItem) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// Cache interface defines caching operations
type Cache interface {
	Get(key string) ([]*utils.FeedItem, bool)
	Set(key string, items []*utils.FeedItem, ttl time.Duration) error
	Delete(key string) error
	Clear() error
}

// InMemoryCache implements an in-memory cache with TTL support
type InMemoryCache struct {
	items map[string]*CacheItem
	mutex sync.RWMutex
	ttl   time.Duration
}

// NewInMemoryCache creates a new in-memory cache
func NewInMemoryCache(defaultTTL time.Duration) *InMemoryCache {
	cache := &InMemoryCache{
		items: make(map[string]*CacheItem),
		ttl:   defaultTTL,
	}

	// Start cleanup goroutine
	go cache.startCleanup()

	return cache
}

// Get retrieves items from cache
func (c *InMemoryCache) Get(key string) ([]*utils.FeedItem, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	item, exists := c.items[key]
	if !exists || item.IsExpired() {
		return nil, false
	}

	return item.Data, true
}

// Set stores items in cache
func (c *InMemoryCache) Set(key string, items []*utils.FeedItem, ttl time.Duration) error {
	if ttl == 0 {
		ttl = c.ttl
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items[key] = &CacheItem{
		Data:      items,
		ExpiresAt: time.Now().Add(ttl),
	}

	return nil
}

// Delete removes an item from cache
func (c *InMemoryCache) Delete(key string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.items, key)
	return nil
}

// Clear removes all items from cache
func (c *InMemoryCache) Clear() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items = make(map[string]*CacheItem)
	return nil
}

// startCleanup periodically removes expired items
func (c *InMemoryCache) startCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup removes expired items
func (c *InMemoryCache) cleanup() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for key, item := range c.items {
		if item.IsExpired() {
			delete(c.items, key)
		}
	}
}

// CacheManager manages caching operations for RSS feeds
type CacheManager struct {
	cache    Cache
	logger   *logrus.Logger
	feedTTL  time.Duration
	itemsTTL time.Duration
	// PerformanceConfig interface to avoid import cycle
	defaultFeedTTL  time.Duration
	defaultItemsTTL time.Duration
	highFreqFeedTTL time.Duration
	lowFreqFeedTTL  time.Duration
}

// NewCacheManager creates a new cache manager
func NewCacheManager(cache Cache, logger *logrus.Logger, defaultFeedTTL, defaultItemsTTL, highFreqFeedTTL, lowFreqFeedTTL time.Duration) *CacheManager {
	return &CacheManager{
		cache:           cache,
		logger:          logger,
		feedTTL:         defaultFeedTTL,
		itemsTTL:        defaultItemsTTL,
		defaultFeedTTL:  defaultFeedTTL,
		defaultItemsTTL: defaultItemsTTL,
		highFreqFeedTTL: highFreqFeedTTL,
		lowFreqFeedTTL:  lowFreqFeedTTL,
	}
}

// GetFeedItems retrieves cached feed items
func (cm *CacheManager) GetFeedItems(url string) ([]*utils.FeedItem, bool) {
	key := fmt.Sprintf("feed:%s", url)
	items, found := cm.cache.Get(key)

	if found {
		cm.logger.WithFields(logrus.Fields{
			"url":         url,
			"items_count": len(items),
		}).Debug("Cache hit for RSS feed")
	} else {
		cm.logger.WithField("url", url).Debug("Cache miss for RSS feed")
	}

	return items, found
}

// SetFeedItems caches feed items with adaptive TTL
func (cm *CacheManager) SetFeedItems(url string, items []*utils.FeedItem) error {
	ttl := cm.calculateAdaptiveTTL(url, items)
	key := fmt.Sprintf("feed:%s", url)
	err := cm.cache.Set(key, items, ttl)

	if err != nil {
		cm.logger.WithFields(logrus.Fields{
			"url":         url,
			"items_count": len(items),
			"ttl_minutes": ttl.Minutes(),
			"error":       err.Error(),
		}).Error("Failed to cache RSS feed")
		return err
	}

	cm.logger.WithFields(logrus.Fields{
		"url":         url,
		"items_count": len(items),
		"ttl_minutes": ttl.Minutes(),
	}).Debug("Cached RSS feed successfully with adaptive TTL")

	return nil
}

// GetStoredItems retrieves cached stored items
func (cm *CacheManager) GetStoredItems(queryKey string) ([]*utils.FeedItem, bool) {
	items, found := cm.cache.Get(queryKey)

	if found {
		cm.logger.WithFields(logrus.Fields{
			"query_key":   queryKey,
			"items_count": len(items),
		}).Debug("Cache hit for stored items")
	} else {
		cm.logger.WithField("query_key", queryKey).Debug("Cache miss for stored items")
	}

	return items, found
}

// SetStoredItems caches stored items
func (cm *CacheManager) SetStoredItems(queryKey string, items []*utils.FeedItem) error {
	err := cm.cache.Set(queryKey, items, cm.itemsTTL)

	if err != nil {
		cm.logger.WithFields(logrus.Fields{
			"query_key":   queryKey,
			"items_count": len(items),
			"error":       err.Error(),
		}).Error("Failed to cache stored items")
		return err
	}

	cm.logger.WithFields(logrus.Fields{
		"query_key":   queryKey,
		"items_count": len(items),
	}).Debug("Cached stored items successfully")

	return nil
}

// InvalidateFeed removes cached feed data
func (cm *CacheManager) InvalidateFeed(url string) error {
	key := fmt.Sprintf("feed:%s", url)
	err := cm.cache.Delete(key)

	if err != nil {
		cm.logger.WithFields(logrus.Fields{
			"url":   url,
			"error": err.Error(),
		}).Error("Failed to invalidate RSS feed cache")
		return err
	}

	cm.logger.WithField("url", url).Debug("Invalidated RSS feed cache")
	return nil
}

// calculateAdaptiveTTL determines optimal cache TTL based on feed characteristics
func (cm *CacheManager) calculateAdaptiveTTL(url string, items []*utils.FeedItem) time.Duration {
	if len(items) == 0 {
		return cm.defaultFeedTTL
	}

	// Analyze feed update frequency based on item publication dates
	updateFrequency := cm.analyzeUpdateFrequency(items)
	feedSize := len(items)

	// Determine TTL based on update frequency and feed size
	switch {
	case updateFrequency <= 1*time.Hour:
		// High-frequency feeds (news, social media)
		return cm.highFreqFeedTTL
	case updateFrequency >= 24*time.Hour:
		// Low-frequency feeds (blogs, weekly updates)
		return cm.lowFreqFeedTTL
	default:
		// Medium frequency - adjust based on feed size
		if feedSize > 100 {
			// Large feeds might be updated less frequently
			return cm.defaultFeedTTL * 2
		} else if feedSize < 10 {
			// Small feeds might be updated more frequently
			return cm.defaultFeedTTL / 2
		}
		return cm.defaultFeedTTL
	}
}

// analyzeUpdateFrequency analyzes the average time between feed item publications
func (cm *CacheManager) analyzeUpdateFrequency(items []*utils.FeedItem) time.Duration {
	if len(items) < 2 {
		return 24 * time.Hour // Default to low frequency for small feeds
	}

	// Parse publication dates and sort by date
	parsedItems := make([]struct {
		item    *utils.FeedItem
		pubTime time.Time
		valid   bool
	}, 0, len(items))

	for _, item := range items {
		if pubTime, err := time.Parse(time.RFC3339, item.PubDate); err == nil {
			parsedItems = append(parsedItems, struct {
				item    *utils.FeedItem
				pubTime time.Time
				valid   bool
			}{item, pubTime, true})
		} else {
			// Try other common date formats
			formats := []string{
				time.RFC1123Z,
				time.RFC1123,
				"2006-01-02 15:04:05",
				"2006-01-02T15:04:05Z",
			}
			for _, format := range formats {
				if pubTime, err := time.Parse(format, item.PubDate); err == nil {
					parsedItems = append(parsedItems, struct {
						item    *utils.FeedItem
						pubTime time.Time
						valid   bool
					}{item, pubTime, true})
					break
				}
			}
		}
	}

	// Filter valid items and sort by publication date (newest first)
	validItems := make([]struct {
		item    *utils.FeedItem
		pubTime time.Time
		valid   bool
	}, 0, len(parsedItems))
	for _, pi := range parsedItems {
		if pi.valid {
			validItems = append(validItems, pi)
		}
	}

	if len(validItems) < 2 {
		return 24 * time.Hour
	}

	// Sort by publication time (newest first)
	sort.Slice(validItems, func(i, j int) bool {
		return validItems[i].pubTime.After(validItems[j].pubTime)
	})

	// Calculate average time difference between consecutive items
	var totalDuration time.Duration
	count := 0

	for i := 1; i < len(validItems); i++ {
		diff := validItems[i-1].pubTime.Sub(validItems[i].pubTime)
		if diff > 0 && diff < 7*24*time.Hour { // Ignore unrealistic gaps
			totalDuration += diff
			count++
		}
	}

	if count == 0 {
		return 24 * time.Hour
	}

	return totalDuration / time.Duration(count)
}

// ClearAll clears all cached data
func (cm *CacheManager) ClearAll() error {
	err := cm.cache.Clear()

	if err != nil {
		cm.logger.WithError(err).Error("Failed to clear cache")
		return err
	}

	cm.logger.Info("Cache cleared successfully")
	return nil
}
