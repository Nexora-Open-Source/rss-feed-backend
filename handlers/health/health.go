// Package health provides health check handlers for the RSS feed backend
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/Nexora-Open-Source/rss-feed-backend/middleware"
	"github.com/Nexora-Open-Source/rss-feed-backend/utils"
	"github.com/sirupsen/logrus"
)

// HealthStatus represents the health check response structure
type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Version   string            `json:"version"`
	Services  map[string]string `json:"services"`
	Uptime    string            `json:"uptime"`
}

// Handler contains dependencies for health handlers
type Handler struct {
	DatastoreClient *datastore.Client
	Logger          *logrus.Logger
}

// NewHandler creates a new health handler
func NewHandler(datastoreClient *datastore.Client, logger *logrus.Logger) *Handler {
	return &Handler{
		DatastoreClient: datastoreClient,
		Logger:          logger,
	}
}

// HandleHealthCheck provides a health check endpoint for monitoring
func (h *Handler) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = utils.GenerateRequestID()
		w.Header().Set("X-Request-ID", requestID)
	}

	health := HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now().Format(time.RFC3339),
		Version:   "1.0.0",
		Services:  make(map[string]string),
		Uptime:    time.Since(startTime).String(),
	}

	// Check Datastore connectivity
	if err := h.checkDatastoreHealth(); err != nil {
		health.Status = "unhealthy"
		health.Services["datastore"] = "unhealthy: " + err.Error()
		h.Logger.WithFields(logrus.Fields{
			"service": "datastore",
			"error":   err.Error(),
		}).Error("Health check failed for datastore")
	} else {
		health.Services["datastore"] = "healthy"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(health)
}

// HandleLivenessCheck provides a simple liveness probe
func (h *Handler) HandleLivenessCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "alive",
		"timestamp": time.Now().Format(time.RFC3339),
		"uptime":    time.Since(startTime).String(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// HandleReadinessCheck provides a readiness probe
func (h *Handler) HandleReadinessCheck(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = utils.GenerateRequestID()
	}

	// Check if essential services are ready
	if err := h.checkDatastoreHealth(); err != nil {
		middleware.RespondServiceUnavailable(w, err, requestID)
		return
	}

	response := map[string]interface{}{
		"status":    "ready",
		"timestamp": time.Now().Format(time.RFC3339),
		"services": map[string]string{
			"datastore": "ready",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// checkDatastoreHealth checks if Datastore is accessible
func (h *Handler) checkDatastoreHealth() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to perform a simple query to test connectivity
	query := datastore.NewQuery("__Namespace").KeysOnly().Limit(1)
	_, err := h.DatastoreClient.GetAll(ctx, query, nil)
	return err
}

var startTime = time.Now()
