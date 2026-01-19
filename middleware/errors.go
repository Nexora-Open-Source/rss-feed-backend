/*
Package middleware provides error handling utilities and structured error responses.
*/
package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// ErrorCode represents different types of application errors
type ErrorCode string

const (
	ErrCodeBadRequest         ErrorCode = "BAD_REQUEST"
	ErrCodeUnauthorized       ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden          ErrorCode = "FORBIDDEN"
	ErrCodeNotFound           ErrorCode = "NOT_FOUND"
	ErrCodeRateLimited        ErrorCode = "RATE_LIMITED"
	ErrCodeInternalError      ErrorCode = "INTERNAL_ERROR"
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeValidation         ErrorCode = "VALIDATION_ERROR"
	ErrCodeExternalAPI        ErrorCode = "EXTERNAL_API_ERROR"
)

// APIError represents a structured error response
type APIError struct {
	Error     ErrorCode `json:"error"`
	Message   string    `json:"message"`
	Details   string    `json:"details,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
	Timestamp string    `json:"timestamp"`
}

// ErrorHandler provides structured error responses
func ErrorHandler(w http.ResponseWriter, err error, code ErrorCode, statusCode int, requestID string) {
	apiErr := APIError{
		Error:     code,
		Message:   getErrorMessage(code),
		Details:   err.Error(),
		RequestID: requestID,
		Timestamp: getCurrentTimestamp(),
	}

	// Log the error with context
	Logger.WithFields(logrus.Fields{
		"error_code":  code,
		"status_code": statusCode,
		"request_id":  requestID,
		"error":       err.Error(),
	}).Error("API error occurred")

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// Send JSON response
	json.NewEncoder(w).Encode(apiErr)
}

// getErrorMessage returns a user-friendly message for each error code
func getErrorMessage(code ErrorCode) string {
	switch code {
	case ErrCodeBadRequest:
		return "The request is invalid or malformed"
	case ErrCodeUnauthorized:
		return "Authentication is required to access this resource"
	case ErrCodeForbidden:
		return "You don't have permission to access this resource"
	case ErrCodeNotFound:
		return "The requested resource was not found"
	case ErrCodeRateLimited:
		return "Rate limit exceeded. Please try again later"
	case ErrCodeInternalError:
		return "An internal server error occurred"
	case ErrCodeServiceUnavailable:
		return "The service is temporarily unavailable"
	case ErrCodeValidation:
		return "Request validation failed"
	case ErrCodeExternalAPI:
		return "Failed to communicate with external service"
	default:
		return "An unknown error occurred"
	}
}

// getCurrentTimestamp returns the current timestamp in ISO format
func getCurrentTimestamp() string {
	return fmt.Sprintf("%d", time.Now().Unix())
}

// Common error response helpers
func RespondBadRequest(w http.ResponseWriter, err error, requestID string) {
	ErrorHandler(w, err, ErrCodeBadRequest, http.StatusBadRequest, requestID)
}

func RespondUnauthorized(w http.ResponseWriter, err error, requestID string) {
	ErrorHandler(w, err, ErrCodeUnauthorized, http.StatusUnauthorized, requestID)
}

func RespondForbidden(w http.ResponseWriter, err error, requestID string) {
	ErrorHandler(w, err, ErrCodeForbidden, http.StatusForbidden, requestID)
}

func RespondNotFound(w http.ResponseWriter, err error, requestID string) {
	ErrorHandler(w, err, ErrCodeNotFound, http.StatusNotFound, requestID)
}

func RespondRateLimited(w http.ResponseWriter, err error, requestID string) {
	ErrorHandler(w, err, ErrCodeRateLimited, http.StatusTooManyRequests, requestID)
}

func RespondInternalError(w http.ResponseWriter, err error, requestID string) {
	ErrorHandler(w, err, ErrCodeInternalError, http.StatusInternalServerError, requestID)
}

func RespondServiceUnavailable(w http.ResponseWriter, err error, requestID string) {
	ErrorHandler(w, err, ErrCodeServiceUnavailable, http.StatusServiceUnavailable, requestID)
}

func RespondValidationError(w http.ResponseWriter, err error, requestID string) {
	ErrorHandler(w, err, ErrCodeValidation, http.StatusBadRequest, requestID)
}

func RespondExternalAPIError(w http.ResponseWriter, err error, requestID string) {
	ErrorHandler(w, err, ErrCodeExternalAPI, http.StatusBadGateway, requestID)
}
