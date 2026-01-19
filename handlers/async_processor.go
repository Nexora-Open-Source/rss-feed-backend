package handlers

import (
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/Nexora-Open-Source/rss-feed-backend/cache"
	"github.com/Nexora-Open-Source/rss-feed-backend/monitoring"
	"github.com/Nexora-Open-Source/rss-feed-backend/types"
	"github.com/Nexora-Open-Source/rss-feed-backend/utils"
	"github.com/sirupsen/logrus"
)

// AsyncJob represents a background job for RSS feed processing
type AsyncJob struct {
	ID        string
	URL       string
	RequestID string
	CreatedAt time.Time
}

// AsyncJobResult represents the result of an async job
type AsyncJobResult struct {
	JobID       string
	URL         string
	Items       []*utils.FeedItem
	Error       error
	ProcessedAt time.Time
	Duration    time.Duration
}

// AsyncProcessor handles background RSS feed processing
type AsyncProcessor struct {
	jobs            chan AsyncJob
	results         chan AsyncJobResult
	quit            chan bool
	wg              sync.WaitGroup
	jobStatus       map[string]*types.AsyncJobStatus
	statusMutex     sync.RWMutex
	logger          *logrus.Logger
	datastoreClient *datastore.Client
	cacheManager    *cache.CacheManager
	// Backpressure configuration
	backpressureEnabled bool
	rejectThreshold     float64
	waitTimeout         time.Duration
	queueSize           int
}

// NewAsyncProcessor creates a new async processor with the given parameters
func NewAsyncProcessor(workers, queueSize int, backpressureEnabled bool, rejectThreshold float64, waitTimeout time.Duration, logger *logrus.Logger, datastoreClient *datastore.Client, cacheManager *cache.CacheManager) *AsyncProcessor {
	processor := &AsyncProcessor{
		jobs:                make(chan AsyncJob, queueSize),
		results:             make(chan AsyncJobResult, queueSize),
		quit:                make(chan bool),
		jobStatus:           make(map[string]*types.AsyncJobStatus),
		logger:              logger,
		datastoreClient:     datastoreClient,
		cacheManager:        cacheManager,
		backpressureEnabled: backpressureEnabled,
		rejectThreshold:     rejectThreshold,
		waitTimeout:         waitTimeout,
		queueSize:           queueSize,
	}

	// Update active workers metric
	monitoring.UpdateActiveWorkers(workers)

	// Start workers
	for i := 0; i < workers; i++ {
		processor.wg.Add(1)
		go processor.worker(i)
	}

	// Start result processor
	processor.wg.Add(1)
	go processor.resultProcessor()

	// Start cleanup goroutine
	go processor.cleanupOldJobs()

	return processor
}

// SubmitJob submits a new job for async processing with backpressure
func (ap *AsyncProcessor) SubmitJob(url, requestID string) (string, error) {
	jobID := fmt.Sprintf("job_%d_%s", time.Now().UnixNano(), requestID)

	job := AsyncJob{
		ID:        jobID,
		URL:       url,
		RequestID: requestID,
		CreatedAt: time.Now(),
	}

	// Initialize job status
	ap.statusMutex.Lock()
	ap.jobStatus[jobID] = &types.AsyncJobStatus{
		JobID:     jobID,
		URL:       url,
		Status:    "pending",
		CreatedAt: job.CreatedAt,
	}
	ap.statusMutex.Unlock()

	// Apply backpressure if enabled
	if ap.backpressureEnabled {
		currentLoad := float64(len(ap.jobs)) / float64(ap.queueSize)
		if currentLoad >= ap.rejectThreshold {
			ap.logger.WithFields(logrus.Fields{
				"url":              url,
				"current_load":     fmt.Sprintf("%.2f", currentLoad),
				"reject_threshold": fmt.Sprintf("%.2f", ap.rejectThreshold),
				"queue_size":       len(ap.jobs),
				"max_queue_size":   ap.queueSize,
			}).Warn("Rejecting job due to backpressure - queue near capacity")
			return "", fmt.Errorf("async processor queue under backpressure (load: %.2f%%)", currentLoad*100)
		}

		// Wait with timeout if queue is getting full
		if currentLoad >= ap.rejectThreshold*0.8 {
			ap.logger.WithFields(logrus.Fields{
				"url":          url,
				"current_load": fmt.Sprintf("%.2f", currentLoad),
				"wait_timeout": ap.waitTimeout.String(),
			}).Info("Queue approaching capacity, applying backpressure delay")
		}
	}

	select {
	case ap.jobs <- job:
		// Update queue size metric
		monitoring.UpdateAsyncQueueSize(len(ap.jobs))

		ap.logger.WithFields(logrus.Fields{
			"job_id":     jobID,
			"url":        url,
			"request_id": requestID,
			"queue_load": fmt.Sprintf("%.2f", float64(len(ap.jobs))/float64(ap.queueSize)),
		}).Info("Job submitted for async processing")
		return jobID, nil
	case <-time.After(ap.waitTimeout):
		ap.logger.WithFields(logrus.Fields{
			"url":            url,
			"wait_timeout":   ap.waitTimeout.String(),
			"queue_size":     len(ap.jobs),
			"max_queue_size": ap.queueSize,
		}).Warn("Job submission timed out due to queue pressure")
		return "", fmt.Errorf("async processor queue timeout after %v", ap.waitTimeout)
	}
}

// GetJobStatus retrieves the status of a job
func (ap *AsyncProcessor) GetJobStatus(jobID string) (*types.AsyncJobStatus, bool) {
	ap.statusMutex.RLock()
	defer ap.statusMutex.RUnlock()

	status, exists := ap.jobStatus[jobID]
	return status, exists
}

// worker processes jobs in the background
func (ap *AsyncProcessor) worker(workerID int) {
	defer ap.wg.Done()

	ap.logger.WithField("worker_id", workerID).Info("Async worker started")

	for {
		select {
		case job := <-ap.jobs:
			// Update queue size metric
			monitoring.UpdateAsyncQueueSize(len(ap.jobs))
			ap.processJob(workerID, job)
		case <-ap.quit:
			ap.logger.WithField("worker_id", workerID).Info("Async worker stopping")
			return
		}
	}
}

// processJob processes a single job
func (ap *AsyncProcessor) processJob(workerID int, job AsyncJob) {
	startTime := time.Now()

	// Update job status to processing
	ap.updateJobStatus(job.ID, "processing", "", 0, 0)

	ap.logger.WithFields(logrus.Fields{
		"worker_id":  workerID,
		"job_id":     job.ID,
		"url":        job.URL,
		"request_id": job.RequestID,
	}).Info("Processing async job")

	// Check cache first
	if ap.cacheManager != nil {
		cachedItems, found := ap.cacheManager.GetFeedItems(job.URL)
		if found {
			result := AsyncJobResult{
				JobID:       job.ID,
				URL:         job.URL,
				Items:       cachedItems,
				Error:       nil,
				ProcessedAt: time.Now(),
				Duration:    time.Since(startTime),
			}

			// Record cache hit and metrics
			monitoring.RecordCacheHit("get_feed_items")
			monitoring.RecordAsyncJob("completed", time.Since(startTime).Seconds())
			monitoring.RecordFeedFetch(job.URL, "cache_hit", time.Since(startTime).Seconds(), len(cachedItems))

			ap.results <- result
			return
		}
		monitoring.RecordCacheMiss("get_feed_items")
	}

	// Fetch RSS feed
	items, err := utils.FetchRSSFeed(job.URL)
	if err != nil {
		result := AsyncJobResult{
			JobID:       job.ID,
			URL:         job.URL,
			Items:       nil,
			Error:       err,
			ProcessedAt: time.Now(),
			Duration:    time.Since(startTime),
		}

		// Record failure metrics
		monitoring.RecordAsyncJob("failed", time.Since(startTime).Seconds())
		monitoring.RecordFeedFetch(job.URL, "failed", time.Since(startTime).Seconds(), -1)

		ap.results <- result
		return
	}

	// Save to datastore
	if err := SaveToDatastore(ap.datastoreClient, items); err != nil {
		ap.logger.WithFields(logrus.Fields{
			"worker_id": workerID,
			"job_id":    job.ID,
			"url":       job.URL,
			"error":     err.Error(),
		}).Error("Failed to save items to datastore in async job")

		result := AsyncJobResult{
			JobID:       job.ID,
			URL:         job.URL,
			Items:       nil,
			Error:       fmt.Errorf("failed to save to datastore: %v", err),
			ProcessedAt: time.Now(),
			Duration:    time.Since(startTime),
		}

		// Record datastore error metrics
		monitoring.RecordDatastoreOperation("save", "failed", time.Since(startTime).Seconds())
		monitoring.RecordAsyncJob("failed", time.Since(startTime).Seconds())

		ap.results <- result
		return
	}

	// Record successful datastore operation
	monitoring.RecordDatastoreOperation("save", "success", time.Since(startTime).Seconds())

	// Cache the results
	if ap.cacheManager != nil {
		if err := ap.cacheManager.SetFeedItems(job.URL, items); err != nil {
			ap.logger.WithFields(logrus.Fields{
				"worker_id": workerID,
				"job_id":    job.ID,
				"url":       job.URL,
				"error":     err.Error(),
			}).Warn("Failed to cache feed items in async job")
			monitoring.RecordDatastoreOperation("cache_set", "failed", 0)
		} else {
			monitoring.RecordDatastoreOperation("cache_set", "success", 0)
		}
	}

	result := AsyncJobResult{
		JobID:       job.ID,
		URL:         job.URL,
		Items:       items,
		Error:       nil,
		ProcessedAt: time.Now(),
		Duration:    time.Since(startTime),
	}

	// Record success metrics
	monitoring.RecordAsyncJob("completed", time.Since(startTime).Seconds())
	monitoring.RecordFeedFetch(job.URL, "success", time.Since(startTime).Seconds(), len(items))

	ap.results <- result

	ap.logger.WithFields(logrus.Fields{
		"worker_id":   workerID,
		"job_id":      job.ID,
		"url":         job.URL,
		"items_count": len(items),
		"duration_ms": time.Since(startTime).Milliseconds(),
	}).Info("Async job completed successfully")
}

// resultProcessor processes job results
func (ap *AsyncProcessor) resultProcessor() {
	defer ap.wg.Done()

	for result := range ap.results {
		status := "completed"
		errorMsg := ""
		itemsCount := len(result.Items)

		if result.Error != nil {
			status = "failed"
			errorMsg = result.Error.Error()
			itemsCount = 0
		}

		ap.updateJobStatus(result.JobID, status, errorMsg, itemsCount, result.Duration.Milliseconds())

		ap.logger.WithFields(logrus.Fields{
			"job_id":      result.JobID,
			"url":         result.URL,
			"status":      status,
			"items_count": itemsCount,
			"duration_ms": result.Duration.Milliseconds(),
		}).Info("Async job result processed")
	}
}

// updateJobStatus updates the status of a job
func (ap *AsyncProcessor) updateJobStatus(jobID, status, errorMsg string, itemsCount int, durationMs int64) {
	ap.statusMutex.Lock()
	defer ap.statusMutex.Unlock()

	if jobStatus, exists := ap.jobStatus[jobID]; exists {
		jobStatus.Status = status
		jobStatus.Error = errorMsg
		jobStatus.ItemsCount = itemsCount
		jobStatus.DurationMs = durationMs
		now := time.Now()
		jobStatus.CompletedAt = &now
	}
}

// cleanupOldJobs removes old job statuses
func (ap *AsyncProcessor) cleanupOldJobs() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		ap.statusMutex.Lock()
		cutoff := time.Now().Add(-24 * time.Hour)
		removed := 0

		for jobID, jobStatus := range ap.jobStatus {
			if jobStatus.CreatedAt.Before(cutoff) {
				delete(ap.jobStatus, jobID)
				removed++
			}
		}

		ap.statusMutex.Unlock()

		if removed > 0 {
			ap.logger.WithField("removed_count", removed).Info("Cleaned up old async job statuses")
		}
	}
}

// Stop gracefully shuts down the async processor
func (ap *AsyncProcessor) Stop() {
	ap.logger.Info("Stopping async processor")
	close(ap.quit)
	close(ap.jobs)
	ap.wg.Wait()
	ap.logger.Info("Async processor stopped")
}

// InitAsyncProcessor initializes the async processor with dependencies
func InitAsyncProcessor(logger *logrus.Logger, datastoreClient *datastore.Client, cacheManager *cache.CacheManager, workers, queueSize int, backpressureEnabled bool, rejectThreshold float64, waitTimeout time.Duration) *AsyncProcessor {
	processor := NewAsyncProcessor(workers, queueSize, backpressureEnabled, rejectThreshold, waitTimeout, logger, datastoreClient, cacheManager)
	logger.WithFields(logrus.Fields{
		"workers":              workers,
		"queue_size":           queueSize,
		"backpressure_enabled": backpressureEnabled,
		"reject_threshold":     rejectThreshold,
		"wait_timeout":         waitTimeout.String(),
	}).Info("Async processor initialized with performance optimizations")
	return processor
}
