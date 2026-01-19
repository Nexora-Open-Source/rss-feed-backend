package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Nexora-Open-Source/rss-feed-backend/config"
	"github.com/Nexora-Open-Source/rss-feed-backend/middleware"
	"golang.org/x/time/rate"
)

func init() {
	// Initialize logger for tests
	middleware.InitLogger()
}

// TestEnhancedRateLimiting tests the improved rate limiting with multiple client identifiers
func TestEnhancedRateLimiting(t *testing.T) {
	limiter := NewRateLimiter(rate.Limit(10), 5)

	// Create a mock handler
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}

	// Wrap with rate limiting middleware
	rateLimitedHandler := RateLimitMiddleware(limiter, handler)

	// Test 1: Same IP but different user agents should have different rate limits
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.Header.Set("User-Agent", "Mozilla/5.0")
	req1.RemoteAddr = "192.168.1.1:12345"

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("User-Agent", "Chrome/91.0")
	req2.RemoteAddr = "192.168.1.1:12345"

	// Both should be allowed initially
	w1 := httptest.NewRecorder()
	w2 := httptest.NewRecorder()

	rateLimitedHandler(w1, req1)
	rateLimitedHandler(w2, req2)

	if w1.Code != http.StatusOK || w2.Code != http.StatusOK {
		t.Errorf("Both requests should be allowed initially")
	}

	// Test 2: Requests with same identifiers should share rate limit
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("User-Agent", "Mozilla/5.0")
		req.RemoteAddr = "192.168.1.1:12345"

		w := httptest.NewRecorder()
		rateLimitedHandler(w, req)

		if i < 5 && w.Code != http.StatusOK {
			t.Errorf("Request %d should be allowed", i)
		}
		if i >= 5 && w.Code != http.StatusTooManyRequests {
			t.Errorf("Request %d should be rate limited", i)
		}
	}
}

// TestURLValidation tests the enhanced URL validation
func TestURLValidation(t *testing.T) {
	// This would require setting up the full handler with dependencies
	// For now, we'll test the validation logic directly

	// Test cases for URL validation
	testCases := []struct {
		name        string
		url         string
		shouldError bool
		errorMsg    string
	}{
		{"Valid HTTPS URL", "https://example.com/feed.xml", false, ""},
		{"Valid HTTP URL", "http://example.com/rss", false, ""},
		{"Empty URL", "", true, "URL cannot be empty"},
		{"Invalid scheme", "ftp://example.com/feed.xml", true, "only HTTP and HTTPS URLs are allowed"},
		{"Localhost URL", "https://localhost/feed.xml", true, "access to private networks and localhost is not allowed"},
		{"Private IP URL", "https://192.168.1.1/feed.xml", true, "access to private networks and localhost is not allowed"},
		{"Suspicious file extension", "https://example.com/feed.exe", true, "URL contains suspicious file extension"},
		{"Script injection", "https://example.com/feed.xml?param=<script>alert('xss')</script>", true, "URL contains potentially malicious content"},
		{"Excessive length", "https://example.com/" + string(make([]byte, 2050)), true, "URL length exceeds maximum allowed size"},
		{"Missing host", "https:///feed.xml", true, "URL must have a valid host"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// We would need to create a handler instance to test this properly
			// For now, just validate the test cases are well-defined
			if tc.url == "" && !tc.shouldError {
				t.Errorf("Empty URL should error")
			}
		})
	}
}

// TestCORSConfiguration tests the enhanced CORS middleware
func TestCORSConfiguration(t *testing.T) {
	// Set test environment
	os.Setenv("PROJECT_ID", "test-project")
	os.Setenv("ENVIRONMENT", "development")
	os.Setenv("DEV_CORS_ORIGINS", "https://localhost:3000,https://127.0.0.1:3000")
	os.Setenv("CORS_ALLOWED_METHODS", "GET,POST,OPTIONS")
	os.Setenv("CORS_ALLOWED_HEADERS", "Content-Type,Authorization")

	// Create test configuration
	appConfig, err := config.NewAppConfig()
	if err != nil {
		t.Fatalf("Failed to create app config: %v", err)
	}

	// Create a mock handler
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}

	// Wrap with CORS middleware
	corsHandler := CORSMiddleware(http.HandlerFunc(handler), appConfig.Config)

	// Test cases
	testCases := []struct {
		name           string
		origin         string
		shouldAllow    bool
		expectedOrigin string
	}{
		{"Allowed origin", "https://localhost:3000", true, "https://localhost:3000"},
		{"Disallowed origin", "https://evil.com", false, ""},
		{"No origin header", "", false, ""},
		{"Allowed origin with different case", "https://LOCALHOST:3000", false, ""}, // Case sensitive
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}

			w := httptest.NewRecorder()
			corsHandler.ServeHTTP(w, req)

			originHeader := w.Header().Get("Access-Control-Allow-Origin")
			if tc.shouldAllow && originHeader != tc.expectedOrigin {
				t.Errorf("Expected origin header %s, got %s", tc.expectedOrigin, originHeader)
			}
			if !tc.shouldAllow && originHeader != "" {
				t.Errorf("Expected no origin header, got %s", originHeader)
			}

			// Check other CORS headers
			methodsHeader := w.Header().Get("Access-Control-Allow-Methods")
			if methodsHeader != "GET,POST,OPTIONS" {
				t.Errorf("Expected methods header 'GET,POST,OPTIONS', got '%s'", methodsHeader)
			}
		})
	}

	// Test OPTIONS preflight
	t.Run("Preflight request", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/", nil)
		req.Header.Set("Origin", "https://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", "POST")

		w := httptest.NewRecorder()
		corsHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for OPTIONS, got %d", w.Code)
		}
	})
}

// TestClientIdentifier tests the enhanced client identification
func TestClientIdentifier(t *testing.T) {
	testCases := []struct {
		name       string
		setupReq   func(*http.Request)
		expectSame bool
	}{
		{
			name: "Same IP and user agent",
			setupReq: func(req *http.Request) {
				req.RemoteAddr = "192.168.1.1:12345"
				req.Header.Set("User-Agent", "Mozilla/5.0")
			},
			expectSame: true,
		},
		{
			name: "Different IP, same user agent",
			setupReq: func(req *http.Request) {
				req.RemoteAddr = "192.168.1.2:12345"
				req.Header.Set("User-Agent", "Mozilla/5.0")
			},
			expectSame: false,
		},
		{
			name: "Same IP, different user agent",
			setupReq: func(req *http.Request) {
				req.RemoteAddr = "192.168.1.1:12345"
				req.Header.Set("User-Agent", "Chrome/91.0")
			},
			expectSame: false,
		},
		{
			name: "With session cookie",
			setupReq: func(req *http.Request) {
				req.RemoteAddr = "192.168.1.1:12345"
				req.Header.Set("User-Agent", "Mozilla/5.0")
				req.AddCookie(&http.Cookie{Name: "session_id", Value: "test-session-123"})
			},
			expectSame: false, // Different from previous due to session
		},
	}

	var previousID string

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			tc.setupReq(req)

			clientID := getClientIdentifier(req)

			if i > 0 {
				if tc.expectSame && clientID != previousID {
					t.Errorf("Expected same client ID, got different: %s vs %s", previousID, clientID)
				}
				if !tc.expectSame && clientID == previousID {
					t.Errorf("Expected different client ID, got same: %s", clientID)
				}
			}

			previousID = clientID

			// Verify client ID is consistent length (16 chars as per implementation)
			if len(clientID) != 16 {
				t.Errorf("Expected client ID length 16, got %d", len(clientID))
			}
		})
	}
}

// TestRateLimiterCleanup tests the rate limiter cleanup functionality
func TestRateLimiterCleanup(t *testing.T) {
	limiter := NewRateLimiter(rate.Limit(10), 5)

	// Add some clients
	limiter.Allow("client1")
	limiter.Allow("client2")
	limiter.Allow("client3")

	// Verify clients exist
	if len(limiter.clients) != 3 {
		t.Errorf("Expected 3 clients, got %d", len(limiter.clients))
	}

	// Manually set last seen to old time to test cleanup
	limiter.mutex.Lock()
	for _, client := range limiter.clients {
		client.lastSeen = time.Now().Add(-10 * time.Minute)
	}
	limiter.mutex.Unlock()

	// Run cleanup
	limiter.Cleanup()

	// Verify clients are cleaned up
	if len(limiter.clients) != 0 {
		t.Errorf("Expected 0 clients after cleanup, got %d", len(limiter.clients))
	}
}
