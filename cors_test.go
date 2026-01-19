package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Nexora-Open-Source/rss-feed-backend/config"
	"github.com/Nexora-Open-Source/rss-feed-backend/middleware"
)

func init() {
	// Initialize logger for tests
	middleware.InitLogger()
}

// TestCORSLogic tests the enhanced CORS middleware logic without full app initialization
func TestCORSLogic(t *testing.T) {
	// Create a test configuration directly
	testConfig := &config.Config{
		CORSConfig: config.CORSConfig{
			Environment: "development",
			DevelopmentOrigins: []string{
				"https://localhost:3000",
				"https://127.0.0.1:3000",
			},
			StagingOrigins: []string{
				"https://staging.example.com",
			},
			ProductionOrigins: []string{
				"https://example.com",
			},
			AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
			AllowedHeaders:   []string{"Content-Type", "Authorization"},
			AllowCredentials: true,
			MaxAge:           86400,
		},
	}

	// Create a mock handler
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}

	// Wrap with CORS middleware
	corsHandler := CORSMiddleware(http.HandlerFunc(handler), testConfig)

	// Test cases
	testCases := []struct {
		name           string
		origin         string
		shouldAllow    bool
		expectedOrigin string
	}{
		{"Allowed development origin", "https://localhost:3000", true, "https://localhost:3000"},
		{"Allowed 127.0.0.1 origin", "https://127.0.0.1:3000", true, "https://127.0.0.1:3000"},
		{"Disallowed origin", "https://evil.com", false, ""},
		{"No origin header", "", false, ""},
		{"Case sensitive check", "https://LOCALHOST:3000", false, ""},
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
			if methodsHeader != "GET, POST, OPTIONS" {
				t.Errorf("Expected methods header 'GET, POST, OPTIONS', got '%s'", methodsHeader)
			}

			credentialsHeader := w.Header().Get("Access-Control-Allow-Credentials")
			if credentialsHeader != "true" {
				t.Errorf("Expected credentials header 'true', got '%s'", credentialsHeader)
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

// TestEnvironmentBasedOrigins tests that origins are selected based on environment
func TestEnvironmentBasedOrigins(t *testing.T) {
	testCases := []struct {
		environment     string
		expectedOrigins []string
	}{
		{"development", []string{"https://localhost:3000", "https://127.0.0.1:3000"}},
		{"dev", []string{"https://localhost:3000", "https://127.0.0.1:3000"}},
		{"staging", []string{"https://staging.example.com"}},
		{"stage", []string{"https://staging.example.com"}},
		{"production", []string{"https://example.com"}},
		{"prod", []string{"https://example.com"}},
		{"unknown", []string{"https://localhost:3000", "https://127.0.0.1:3000"}}, // Falls back to development
	}

	for _, tc := range testCases {
		t.Run(tc.environment, func(t *testing.T) {
			corsConfig := config.CORSConfig{
				Environment:        tc.environment,
				DevelopmentOrigins: []string{"https://localhost:3000", "https://127.0.0.1:3000"},
				StagingOrigins:     []string{"https://staging.example.com"},
				ProductionOrigins:  []string{"https://example.com"},
			}

			origins := getAllowedOrigins(corsConfig)
			if len(origins) != len(tc.expectedOrigins) {
				t.Errorf("Expected %d origins, got %d", len(tc.expectedOrigins), len(origins))
			}

			for i, expected := range tc.expectedOrigins {
				if i >= len(origins) || origins[i] != expected {
					t.Errorf("Expected origin %s at index %d, got %s", expected, i, origins[i])
				}
			}
		})
	}
}

// TestSubdomainValidation tests subdomain validation logic
func TestSubdomainValidation(t *testing.T) {
	corsConfig := config.CORSConfig{
		AllowSubdomains:    true,
		AllowedDomains:     []string{"example.com", "trusted.com"},
		DevelopmentOrigins: []string{"*.example.com"},
	}

	testCases := []struct {
		name        string
		origin      string
		shouldAllow bool
	}{
		{"Exact domain match", "https://example.com", true},
		{"Subdomain match", "https://api.example.com", true},
		{"Different subdomain", "https://test.example.com", true},
		{"Trusted domain", "https://trusted.com", true},
		{"Trusted subdomain", "https://api.trusted.com", true},
		{"Unrelated domain", "https://evil.com", false},
		{"Similar but different", "https://example.com.evil.com", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			allowed := isOriginAllowed(tc.origin, corsConfig)
			if allowed != tc.shouldAllow {
				t.Errorf("Expected %v for origin %s, got %v", tc.shouldAllow, tc.origin, allowed)
			}
		})
	}
}
