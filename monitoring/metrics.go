// Package monitoring provides metrics and observability for the RSS feed backend
package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Feed fetching metrics
	feedFetchTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rss_feed_fetch_total",
			Help: "Total number of RSS feed fetch attempts",
		},
		[]string{"url", "status"},
	)

	feedFetchDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rss_feed_fetch_duration_seconds",
			Help:    "Duration of RSS feed fetch operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"url", "status"},
	)

	feedItemsCount = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rss_feed_items_count",
			Help:    "Number of items fetched from RSS feeds",
			Buckets: []float64{0, 1, 5, 10, 25, 50, 100, 250, 500, 1000},
		},
		[]string{"url"},
	)

	// Async processor metrics
	asyncJobsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rss_async_jobs_total",
			Help: "Total number of async jobs processed",
		},
		[]string{"status"},
	)

	asyncJobDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rss_async_job_duration_seconds",
			Help:    "Duration of async job processing",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"},
	)

	asyncQueueSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "rss_async_queue_size",
			Help: "Current size of async job queue",
		},
	)

	// Cache metrics
	cacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rss_cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"operation"},
	)

	cacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rss_cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"operation"},
	)

	// Datastore metrics
	datastoreOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rss_datastore_operations_total",
			Help: "Total number of datastore operations",
		},
		[]string{"operation", "status"},
	)

	datastoreOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rss_datastore_operation_duration_seconds",
			Help:    "Duration of datastore operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "status"},
	)

	// HTTP metrics
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rss_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rss_http_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint", "status"},
	)

	// System metrics
	activeWorkers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "rss_active_workers",
			Help: "Number of active async workers",
		},
	)
)

// RecordFeedFetch records metrics for RSS feed fetching
func RecordFeedFetch(url, status string, duration float64, itemsCount int) {
	feedFetchTotal.WithLabelValues(url, status).Inc()
	feedFetchDuration.WithLabelValues(url, status).Observe(duration)
	if itemsCount >= 0 {
		feedItemsCount.WithLabelValues(url).Observe(float64(itemsCount))
	}
}

// RecordAsyncJob records metrics for async job processing
func RecordAsyncJob(status string, duration float64) {
	asyncJobsTotal.WithLabelValues(status).Inc()
	asyncJobDuration.WithLabelValues(status).Observe(duration)
}

// UpdateAsyncQueueSize updates the async queue size gauge
func UpdateAsyncQueueSize(size int) {
	asyncQueueSize.Set(float64(size))
}

// RecordCacheHit records a cache hit
func RecordCacheHit(operation string) {
	cacheHits.WithLabelValues(operation).Inc()
}

// RecordCacheMiss records a cache miss
func RecordCacheMiss(operation string) {
	cacheMisses.WithLabelValues(operation).Inc()
}

// RecordDatastoreOperation records datastore operation metrics
func RecordDatastoreOperation(operation, status string, duration float64) {
	datastoreOperations.WithLabelValues(operation, status).Inc()
	datastoreOperationDuration.WithLabelValues(operation, status).Observe(duration)
}

// RecordHTTPRequest records HTTP request metrics
func RecordHTTPRequest(method, endpoint, status string, duration float64) {
	httpRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
	httpRequestDuration.WithLabelValues(method, endpoint, status).Observe(duration)
}

// UpdateActiveWorkers updates the active workers gauge
func UpdateActiveWorkers(count int) {
	activeWorkers.Set(float64(count))
}
