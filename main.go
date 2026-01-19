/*
Package main initializes the RSS Feed backend server.

This backend provides APIs to fetch and store RSS feed data and retrieve predefined feed sources.
It uses Google Cloud Datastore for storage and supports parsing RSS feeds via the `gofeed` library.

Key Features:
  - Fetch RSS feeds from a URL.
  - Store parsed feeds in Google Cloud Datastore.
  - Retrieve predefined or categorized RSS feed sources.
  - Designed for easy integration with frontend applications.

Run the application:

	$ go run main.go

Endpoints:
  - GET /fetch-store?url=<rss-url>: Fetch and store RSS feed data.
  - GET /feeds: Retrieve predefined RSS feed sources.
*/
package main

import (
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Nexora-Open-Source/rss-feed-backend/config"
	_ "github.com/Nexora-Open-Source/rss-feed-backend/docs"
	"github.com/Nexora-Open-Source/rss-feed-backend/middleware"
	"github.com/Nexora-Open-Source/rss-feed-backend/monitoring"
	"github.com/Nexora-Open-Source/rss-feed-backend/utils"
	"github.com/gorilla/mux"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"golang.org/x/time/rate"
)

// RateLimiter implements a simple token bucket rate limiter
type RateLimiter struct {
	clients map[string]*ClientLimiter
	mutex   sync.RWMutex
	rate    rate.Limit
	burst   int
}

// ClientLimiter represents a rate limiter for a specific client
type ClientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(r rate.Limit, b int) *RateLimiter {
	return &RateLimiter{
		clients: make(map[string]*ClientLimiter),
		rate:    r,
		burst:   b,
	}
}

// Allow checks if a client is allowed to make a request
func (rl *RateLimiter) Allow(clientID string) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	if _, exists := rl.clients[clientID]; !exists {
		rl.clients[clientID] = &ClientLimiter{
			limiter:  rate.NewLimiter(rl.rate, rl.burst),
			lastSeen: time.Now(),
		}
	}

	rl.clients[clientID].lastSeen = time.Now()
	return rl.clients[clientID].limiter.Allow()
}

// Cleanup removes stale client entries
func (rl *RateLimiter) Cleanup() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	for clientID, client := range rl.clients {
		if time.Since(client.lastSeen) > 5*time.Minute {
			delete(rl.clients, clientID)
		}
	}
}

func main() {
	// Initialize tracing
	tracerProvider, err := monitoring.InitTracing("rss-feed-backend")
	if err != nil {
		log.Fatalf("Failed to initialize tracing: %v", err)
	}
	defer monitoring.ShutdownTracing(tracerProvider)

	// Initialize alert manager
	alertManager := monitoring.NewAlertManager(middleware.Logger)
	defer alertManager.Stop()

	// Initialize configuration and services
	appConfig, err := config.NewAppConfig()
	if err != nil {
		log.Fatalf("Failed to initialize application configuration: %v", err)
	}
	defer appConfig.Services.Close()

	// Initialize structured logger
	middleware.InitLogger()
	middleware.Logger.Info("Starting RSS Feed Backend Server")

	// Initialize handler with dependencies using DI container
	handler, err := appConfig.Services.Container.GetHandler()
	if err != nil {
		log.Fatalf("Failed to initialize handler: %v", err)
	}

	// Initialize rate limiter with configuration
	limiter := NewRateLimiter(rate.Limit(appConfig.Config.RateLimitRequestsPerMinute/60.0), appConfig.Config.RateLimitBurst)

	// Start cleanup goroutine with configured interval
	go func() {
		ticker := time.NewTicker(appConfig.Config.ClientCleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			limiter.Cleanup()
		}
	}()

	// Initialize the router
	router := mux.NewRouter()

	// Setup metrics endpoint
	monitoring.SetupMetricsEndpoint(router)

	// Setup health check endpoints (no rate limiting)
	router.HandleFunc("/health", handler.HandleHealthCheck).Methods("GET")
	router.HandleFunc("/health/live", handler.HandleLivenessCheck).Methods("GET")
	router.HandleFunc("/health/ready", handler.HandleReadinessCheck).Methods("GET")

	// Setup Swagger documentation
	router.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

	// Setup API routes with rate limiting and monitoring middleware
	router.HandleFunc("/fetch-store", MonitoringMiddleware(RateLimitMiddleware(limiter, handler.HandleFetchAndStore))).Methods("POST")
	router.HandleFunc("/feeds", MonitoringMiddleware(RateLimitMiddleware(limiter, handler.HandleGetFeeds))).Methods("GET")
	router.HandleFunc("/items", MonitoringMiddleware(RateLimitMiddleware(limiter, handler.HandleGetFeedItems))).Methods("GET")
	router.HandleFunc("/items/legacy", MonitoringMiddleware(RateLimitMiddleware(limiter, handler.HandleGetFeedItemsLegacy))).Methods("GET")
	router.HandleFunc("/job-status", MonitoringMiddleware(RateLimitMiddleware(limiter, handler.HandleGetJobStatus))).Methods("GET")

	// Apply logging middleware
	withLogging := middleware.LoggingMiddleware(router)

	// Attach the CORS middleware with enhanced configuration
	withCORS := CORSMiddleware(withLogging, appConfig.Config)

	// Start the server
	fmt.Println("Server is running on https://localhost:8080")
	fmt.Println("Metrics available at http://localhost:8080/metrics")
	middleware.Logger.Info("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", withCORS))
}

// MonitoringMiddleware adds metrics and tracing to HTTP handlers
func MonitoringMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create tracing span
		ctx, span := monitoring.CreateSpan(r.Context(), fmt.Sprintf("%s %s", r.Method, r.URL.Path))
		defer span.End()

		// Set span attributes
		monitoring.SetSpanAttributes(span, map[string]interface{}{
			"http.method":     r.Method,
			"http.url":        r.URL.String(),
			"http.user_agent": r.UserAgent(),
			"remote.addr":     r.RemoteAddr,
		})

		// Update request context with tracing
		r = r.WithContext(ctx)

		// Wrap response writer to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: 200}

		// Call the next handler
		next.ServeHTTP(rw, r)

		// Record metrics
		duration := time.Since(start).Seconds()
		status := fmt.Sprintf("%d", rw.statusCode)

		monitoring.RecordHTTPRequest(r.Method, r.URL.Path, status, duration)

		// Update span with response info
		monitoring.SetSpanAttributes(span, map[string]interface{}{
			"http.status_code": rw.statusCode,
			"duration_seconds": duration,
		})

		// Record error if status indicates failure
		if rw.statusCode >= 400 {
			monitoring.SetSpanError(span, fmt.Errorf("HTTP %d", rw.statusCode))
		}
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// getClientIdentifier generates a robust client identifier using multiple factors
func getClientIdentifier(r *http.Request) string {
	var identifiers []string

	// 1. IP Address (with X-Forwarded-For support)
	ip := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// Take the first IP from the forwarded chain
		ips := strings.Split(forwarded, ",")
		ip = strings.TrimSpace(ips[0])
	} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		ip = realIP
	}
	identifiers = append(identifiers, "ip:"+ip)

	// 2. User Agent (normalized)
	userAgent := r.Header.Get("User-Agent")
	if userAgent != "" {
		// Normalize user agent by removing version numbers and extra spaces
		userAgent = strings.ToLower(userAgent)
		userAgent = strings.Fields(userAgent)[0] // Take first word
		identifiers = append(identifiers, "ua:"+userAgent)
	}

	// 3. Accept-Language header
	if acceptLang := r.Header.Get("Accept-Language"); acceptLang != "" {
		// Take first two characters of language code
		lang := strings.ToLower(strings.TrimSpace(acceptLang[:2]))
		identifiers = append(identifiers, "lang:"+lang)
	}

	// 4. Session/Cookie identifier (if available)
	if cookie, err := r.Cookie("session_id"); err == nil && cookie.Value != "" {
		// Hash the cookie value for privacy
		hash := sha256.Sum256([]byte(cookie.Value))
		identifiers = append(identifiers, "sess:"+fmt.Sprintf("%x", hash)[:8])
	}

	// Combine all identifiers
	combined := strings.Join(identifiers, "|")

	// Create final hash for client ID
	finalHash := sha256.Sum256([]byte(combined))
	return fmt.Sprintf("%x", finalHash)[:16]
}

// RateLimitMiddleware implements enhanced rate limiting for HTTP handlers
func RateLimitMiddleware(limiter *RateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Use robust client identifier instead of just IP
		clientID := getClientIdentifier(r)

		if !limiter.Allow(clientID) {
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = utils.GenerateRequestID()
			}
			middleware.RespondRateLimited(w, fmt.Errorf("rate limit exceeded"), requestID)
			return
		}

		next.ServeHTTP(w, r)
	}
}

// getAllowedOrigins returns the appropriate allowed origins based on environment
func getAllowedOrigins(corsConfig config.CORSConfig) []string {
	switch strings.ToLower(corsConfig.Environment) {
	case "production", "prod":
		return corsConfig.ProductionOrigins
	case "staging", "stage":
		return corsConfig.StagingOrigins
	case "development", "dev", "local":
		return corsConfig.DevelopmentOrigins
	default:
		return corsConfig.DevelopmentOrigins
	}
}

// isOriginAllowed checks if the origin is allowed based on CORS configuration
func isOriginAllowed(origin string, corsConfig config.CORSConfig) bool {
	allowedOrigins := getAllowedOrigins(corsConfig)

	// Check exact matches first
	for _, allowedOrigin := range allowedOrigins {
		if origin == allowedOrigin {
			return true
		}
	}

	// If subdomains are allowed, check domain patterns
	if corsConfig.AllowSubdomains {
		// Check against explicitly allowed domains
		for _, domain := range corsConfig.AllowedDomains {
			if origin == "https://"+domain || origin == "http://"+domain {
				return true
			}
			if strings.HasSuffix(origin, "."+domain) {
				return true
			}
		}

		// Also check if origin matches any allowed origin with wildcard subdomain
		for _, allowedOrigin := range allowedOrigins {
			if strings.HasPrefix(allowedOrigin, "*.") {
				domain := allowedOrigin[2:] // Remove "*."
				if origin == "https://"+domain || origin == "http://"+domain {
					return true
				}
				if strings.HasSuffix(origin, "."+domain) {
					return true
				}
			}
		}
	}

	return false
}

func CORSMiddleware(next http.Handler, appConfig *config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		corsConfig := appConfig.CORSConfig

		// Set CORS headers based on configuration
		if origin != "" && isOriginAllowed(origin, corsConfig) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		// Set allowed methods
		if len(corsConfig.AllowedMethods) > 0 {
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(corsConfig.AllowedMethods, ", "))
		} else {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}

		// Set allowed headers
		if len(corsConfig.AllowedHeaders) > 0 {
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(corsConfig.AllowedHeaders, ", "))
		} else {
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-Request-ID")
		}

		// Set exposed headers
		if len(corsConfig.ExposedHeaders) > 0 {
			w.Header().Set("Access-Control-Expose-Headers", strings.Join(corsConfig.ExposedHeaders, ", "))
		}

		// Set credentials
		if corsConfig.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Set max age
		if corsConfig.MaxAge > 0 {
			w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", corsConfig.MaxAge))
		}

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
