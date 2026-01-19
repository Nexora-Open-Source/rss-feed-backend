/*
Package config provides configuration management for the RSS Feed backend.

This package separates configuration concerns from business logic and provides
a centralized way to manage application configuration including database
connections, caching, and other service dependencies.
*/
package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/Nexora-Open-Source/rss-feed-backend/cache"
	"github.com/Nexora-Open-Source/rss-feed-backend/container"
	"github.com/Nexora-Open-Source/rss-feed-backend/middleware"
	"github.com/sirupsen/logrus"
)

// Config holds all application configuration
type Config struct {
	ProjectID  string
	LogLevel   string
	ServerPort string
	// Rate limiting configuration
	RateLimitRequestsPerMinute float64
	RateLimitBurst             int
	RateLimitCleanupInterval   time.Duration
	// Enhanced CORS configuration
	CORSConfig CORSConfig
	// Cleanup intervals
	ClientCleanupInterval time.Duration
	// Performance optimization settings
	PerformanceConfig PerformanceConfig
}

// PerformanceConfig holds performance-related configuration
type PerformanceConfig struct {
	// Cache TTL settings
	DefaultFeedTTL  time.Duration `json:"default_feed_ttl"`
	DefaultItemsTTL time.Duration `json:"default_items_ttl"`
	HighFreqFeedTTL time.Duration `json:"high_freq_feed_ttl"`
	LowFreqFeedTTL  time.Duration `json:"low_freq_feed_ttl"`
	// Batch size settings
	DefaultBatchSize   int `json:"default_batch_size"`
	LargeFeedBatchSize int `json:"large_feed_batch_size"`
	SmallFeedBatchSize int `json:"small_feed_batch_size"`
	MaxBatchSize       int `json:"max_batch_size"`
	MinBatchSize       int `json:"min_batch_size"`
	// Async processor settings
	AsyncWorkers         int           `json:"async_workers"`
	AsyncQueueSize       int           `json:"async_queue_size"`
	AsyncBackpressure    bool          `json:"async_backpressure"`
	AsyncRejectThreshold float64       `json:"async_reject_threshold"`
	AsyncWaitTimeout     time.Duration `json:"async_wait_timeout"`
}

// CORSConfig holds CORS-related configuration
type CORSConfig struct {
	// Environment-specific settings
	Environment string
	// Allowed origins based on environment
	DevelopmentOrigins []string
	StagingOrigins     []string
	ProductionOrigins  []string
	// Additional CORS settings
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
	// Dynamic origin validation
	AllowSubdomains bool
	AllowedDomains  []string
}

// Services holds all service dependencies
type Services struct {
	Container *container.Container
	Logger    *logrus.Logger
}

// AppConfig holds both configuration and services
type AppConfig struct {
	Config   *Config
	Services *Services
}

// NewConfig creates a new configuration instance
func NewConfig() *Config {
	environment := getEnv("ENVIRONMENT", "development")

	return &Config{
		ProjectID:  getEnv("PROJECT_ID", ""),
		LogLevel:   getEnv("LOG_LEVEL", "info"),
		ServerPort: getEnv("SERVER_PORT", "8080"),
		// Rate limiting defaults (10 requests per minute, burst of 5)
		RateLimitRequestsPerMinute: getEnvFloat("RATE_LIMIT_RPM", 10.0),
		RateLimitBurst:             getEnvInt("RATE_LIMIT_BURST", 5),
		RateLimitCleanupInterval:   getEnvDuration("RATE_LIMIT_CLEANUP_INTERVAL", 5*time.Minute),
		// Enhanced CORS configuration
		CORSConfig: CORSConfig{
			Environment: environment,
			DevelopmentOrigins: getEnvSlice("DEV_CORS_ORIGINS", []string{
				"http://localhost:3000",
				"http://localhost:3001",
				"http://127.0.0.1:3000",
				"http://127.0.0.1:3001",
				"http://localhost:8080",
			}),
			StagingOrigins: getEnvSlice("STAGING_CORS_ORIGINS", []string{
				"https://staging.yourdomain.com",
				"https://staging-api.yourdomain.com",
			}),
			ProductionOrigins: getEnvSlice("PROD_CORS_ORIGINS", []string{
				"https://yourdomain.com",
				"https://www.yourdomain.com",
				"https://api.yourdomain.com",
			}),
			AllowedMethods: getEnvSlice("CORS_ALLOWED_METHODS", []string{
				"GET", "POST", "PUT", "DELETE", "OPTIONS",
			}),
			AllowedHeaders: getEnvSlice("CORS_ALLOWED_HEADERS", []string{
				"Content-Type", "Authorization", "X-Requested-With",
				"X-Request-ID", "Accept", "Origin", "Cache-Control",
			}),
			ExposedHeaders: getEnvSlice("CORS_EXPOSED_HEADERS", []string{
				"X-Request-ID", "X-Total-Count", "X-Cache",
			}),
			AllowCredentials: getEnvBool("CORS_ALLOW_CREDENTIALS", true),
			MaxAge:           getEnvInt("CORS_MAX_AGE", 86400), // 24 hours
			AllowSubdomains:  getEnvBool("CORS_ALLOW_SUBDOMAINS", false),
			AllowedDomains:   getEnvSlice("CORS_ALLOWED_DOMAINS", []string{}),
		},
		// Cleanup intervals
		ClientCleanupInterval: getEnvDuration("CLIENT_CLEANUP_INTERVAL", 1*time.Minute),
		// Performance optimization settings
		PerformanceConfig: PerformanceConfig{
			// Cache TTL settings (adaptive based on feed frequency)
			DefaultFeedTTL:  getEnvDuration("DEFAULT_FEED_TTL", 15*time.Minute),
			DefaultItemsTTL: getEnvDuration("DEFAULT_ITEMS_TTL", 30*time.Minute),
			HighFreqFeedTTL: getEnvDuration("HIGH_FREQ_FEED_TTL", 5*time.Minute), // For frequently updated feeds
			LowFreqFeedTTL:  getEnvDuration("LOW_FREQ_FEED_TTL", 60*time.Minute), // For rarely updated feeds
			// Batch size settings (adaptive based on feed size)
			DefaultBatchSize:   getEnvInt("DEFAULT_BATCH_SIZE", 500),
			LargeFeedBatchSize: getEnvInt("LARGE_FEED_BATCH_SIZE", 1000), // For feeds with many items
			SmallFeedBatchSize: getEnvInt("SMALL_FEED_BATCH_SIZE", 100),  // For feeds with few items
			MaxBatchSize:       getEnvInt("MAX_BATCH_SIZE", 2000),
			MinBatchSize:       getEnvInt("MIN_BATCH_SIZE", 50),
			// Async processor settings
			AsyncWorkers:         getEnvInt("ASYNC_WORKERS", 3),
			AsyncQueueSize:       getEnvInt("ASYNC_QUEUE_SIZE", 50),
			AsyncBackpressure:    getEnvBool("ASYNC_BACKPRESSURE", true),
			AsyncRejectThreshold: getEnvFloat("ASYNC_REJECT_THRESHOLD", 0.8), // Reject at 80% capacity
			AsyncWaitTimeout:     getEnvDuration("ASYNC_WAIT_TIMEOUT", 5*time.Second),
		},
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.ProjectID == "" {
		return fmt.Errorf("PROJECT_ID environment variable is required")
	}
	return nil
}

// NewServices creates and initializes all service dependencies using DI container
func NewServices(config *Config) (*Services, error) {
	logger := middleware.Logger
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.InfoLevel)
	}

	// Initialize Datastore client
	datastoreClient, err := datastore.NewClient(context.Background(), config.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Datastore client: %v", err)
	}
	logger.WithField("project_id", config.ProjectID).Info("Datastore client initialized successfully")

	// Initialize cache
	inMemoryCache := cache.NewInMemoryCache(30 * time.Minute)
	cacheManager := cache.NewCacheManager(
		inMemoryCache,
		logger,
		config.PerformanceConfig.DefaultFeedTTL,
		config.PerformanceConfig.DefaultItemsTTL,
		config.PerformanceConfig.HighFreqFeedTTL,
		config.PerformanceConfig.LowFreqFeedTTL,
	)
	logger.Info("Cache manager initialized successfully")

	// Initialize dependency injection container
	diContainer := container.NewContainer()
	if err := diContainer.InitializeServices(datastoreClient, cacheManager, logger); err != nil {
		return nil, fmt.Errorf("failed to initialize dependency container: %v", err)
	}

	return &Services{
		Container: diContainer,
		Logger:    logger,
	}, nil
}

// NewAppConfig creates a new application configuration with all dependencies
func NewAppConfig() (*AppConfig, error) {
	config := NewConfig()

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %v", err)
	}

	services, err := NewServices(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize services: %v", err)
	}

	return &AppConfig{
		Config:   config,
		Services: services,
	}, nil
}

// Close gracefully closes all service connections
func (s *Services) Close() error {
	if s.Container != nil {
		return s.Container.Close()
	}
	return nil
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvFloat gets an environment variable as float64 with a default value
func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseFloat(value, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// getEnvInt gets an environment variable as int with a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// getEnvDuration gets an environment variable as time.Duration with a default value
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// getEnvBool gets an environment variable as bool with a default value
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// getEnvSlice gets an environment variable as a string slice with a default value
func getEnvSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}
