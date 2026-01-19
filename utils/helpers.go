/*
Package utils provides helper functions for the RSS feed backend.
*/
package utils

import (
	"crypto/rand"
	"encoding/base64"
	"time"
)

// GenerateRequestID generates a unique request ID
func GenerateRequestID() string {
	return time.Now().Format("20060102150405") + "-" + RandomString(8)
}

// RandomString generates a random string of specified length
func RandomString(length int) string {
	if length <= 0 {
		return ""
	}

	// Generate random bytes
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		// Fallback to time-based generation if crypto/rand fails
		const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		b := make([]byte, length)
		for i := range b {
			b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		}
		return string(b)
	}

	// Encode to base64 and trim to desired length
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}
