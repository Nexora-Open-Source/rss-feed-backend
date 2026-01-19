/*
Package handlers provides HTTP handlers with dependency injection support.

This package defines the Handler struct that contains all service dependencies,
eliminating global variables and enabling better testability and separation of concerns.
*/
package handlers

import (
	"context"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/Nexora-Open-Source/rss-feed-backend/cache"
	"github.com/Nexora-Open-Source/rss-feed-backend/types"
	"github.com/Nexora-Open-Source/rss-feed-backend/utils"
	"github.com/sirupsen/logrus"
)

// CacheManagerInterface defines the interface for cache operations
type CacheManagerInterface interface {
	GetStoredItems(key string) ([]*utils.FeedItem, bool)
	SetStoredItems(key string, items []*utils.FeedItem) error
	GetFeedItems(key string) ([]*utils.FeedItem, bool)
	SetFeedItems(key string, items []*utils.FeedItem) error
}

// AsyncProcessorInterface defines the interface for async processing
type AsyncProcessorInterface interface {
	SubmitJob(url, requestID string) (string, error)
	GetJobStatus(jobID string) (*types.AsyncJobStatus, bool)
}

// DatastoreReaderInterface defines read operations for datastore
type DatastoreReaderInterface interface {
	Get(ctx context.Context, key *datastore.Key, dst interface{}) error
	GetAll(ctx context.Context, q *datastore.Query, dst interface{}) ([]*datastore.Key, error)
}

// DatastoreWriterInterface defines write operations for datastore
type DatastoreWriterInterface interface {
	PutMulti(ctx context.Context, keys []*datastore.Key, src interface{}) ([]*datastore.Key, error)
	DeleteMulti(ctx context.Context, keys []*datastore.Key) error
}

// DatastoreClientInterface combines read and write operations
type DatastoreClientInterface interface {
	DatastoreReaderInterface
	DatastoreWriterInterface
}

// Handler contains all service dependencies for HTTP handlers
type Handler struct {
	DatastoreClient DatastoreClientInterface
	CacheManager    CacheManagerInterface
	Logger          *logrus.Logger
	AsyncProcessor  AsyncProcessorInterface
}

// NewHandler creates a new handler instance with injected dependencies
func NewHandler(datastoreClient *datastore.Client, cacheManager *cache.CacheManager, logger *logrus.Logger) *Handler {
	// Default performance settings for backward compatibility
	asyncProcessor := NewAsyncProcessor(
		3,             // workers
		50,            // queueSize
		true,          // backpressureEnabled
		0.8,           // rejectThreshold (80%)
		5*time.Second, // waitTimeout
		logger,
		datastoreClient,
		cacheManager,
	)
	return &Handler{
		DatastoreClient: datastoreClient,
		CacheManager:    cacheManager,
		Logger:          logger,
		AsyncProcessor:  asyncProcessor,
	}
}

// DatastoreService provides datastore operations
type DatastoreService struct {
	client *datastore.Client
}

// NewDatastoreService creates a new datastore service
func NewDatastoreService(client *datastore.Client) *DatastoreService {
	return &DatastoreService{
		client: client,
	}
}

// CacheService provides cache operations
type CacheService struct {
	manager *cache.CacheManager
}

// NewCacheService creates a new cache service
func NewCacheService(manager *cache.CacheManager) *CacheService {
	return &CacheService{
		manager: manager,
	}
}

// JobStatus represents the status of an async job
type JobStatus struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Progress  int       `json:"progress"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
