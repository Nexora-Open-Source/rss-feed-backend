/*
Package middleware provides HTTP middleware for logging, error handling, and request/response tracking.
*/
package middleware

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// Logger is the global structured logger
var Logger *logrus.Logger

// ResponseWriter captures response data for logging
type ResponseWriter struct {
	http.ResponseWriter
	status int
	body   *bytes.Buffer
}

func (rw *ResponseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *ResponseWriter) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

// InitLogger initializes the structured logger
func InitLogger() {
	Logger = logrus.New()
	Logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})
	Logger.SetLevel(logrus.InfoLevel)
}

// LoggingMiddleware logs HTTP requests and responses
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Read request body for logging
		var bodyBytes []byte
		if r.Body != nil {
			bodyBytes, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// Create response writer to capture response
		rw := &ResponseWriter{
			ResponseWriter: w,
			status:         http.StatusOK,
			body:           bytes.NewBuffer(nil),
		}

		// Process request
		next.ServeHTTP(rw, r)

		// Calculate duration
		duration := time.Since(start)

		// Log request and response
		fields := logrus.Fields{
			"method":      r.Method,
			"path":        r.URL.Path,
			"query":       r.URL.RawQuery,
			"remote_addr": r.RemoteAddr,
			"user_agent":  r.UserAgent(),
			"status":      rw.status,
			"duration_ms": duration.Milliseconds(),
			"request_id":  generateRequestID(),
		}

		// Add request body if present (limit size for security)
		if len(bodyBytes) > 0 && len(bodyBytes) < 1024 {
			fields["request_body"] = string(bodyBytes)
		}

		// Add response body for errors (limit size)
		if rw.status >= 400 && rw.body.Len() > 0 && rw.body.Len() < 1024 {
			fields["response_body"] = rw.body.String()
		}

		// Log with appropriate level based on status
		switch {
		case rw.status >= 500:
			Logger.WithFields(fields).Error("Request completed with server error")
		case rw.status >= 400:
			Logger.WithFields(fields).Warn("Request completed with client error")
		default:
			Logger.WithFields(fields).Info("Request completed successfully")
		}
	})
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random string of specified length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
