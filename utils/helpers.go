/*
Package utils provides helper functions for the RSS feed backend.
*/
package utils

import (
	"time"
)

// GenerateRequestID generates a unique request ID
func GenerateRequestID() string {
	return time.Now().Format("20060102150405") + "-" + RandomString(8)
}

// RandomString generates a random string of specified length
func RandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
