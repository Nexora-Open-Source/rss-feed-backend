package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Nexora-Open-Source/rss-feed-backend/middleware"
	"github.com/Nexora-Open-Source/rss-feed-backend/utils"
	"github.com/sirupsen/logrus"
)

/*
HandleGetJobStatus retrieves the status of an async job.

Query Parameters:
  - job_id: The ID of the job to check.

Example:

	GET /job-status?job_id=job_1234567890_abc123

Response:
  - 200 OK: Job status information.
  - 400 Bad Request: Missing job_id parameter.
  - 404 Not Found: Job not found.
*/
func (h *Handler) HandleGetJobStatus(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = utils.GenerateRequestID()
		w.Header().Set("X-Request-ID", requestID)
	}

	// Get job ID from query params
	jobID := r.URL.Query().Get("job_id")
	if jobID == "" {
		middleware.RespondBadRequest(w, fmt.Errorf("job_id parameter is missing"), requestID)
		return
	}

	// Log the request
	middleware.Logger.WithFields(logrus.Fields{
		"request_id": requestID,
		"job_id":     jobID,
		"action":     "get_job_status",
	}).Info("Processing job status request")

	// Get job status from async processor
	jobStatus, exists := h.AsyncProcessor.GetJobStatus(jobID)
	if !exists {
		middleware.RespondNotFound(w, fmt.Errorf("job not found"), requestID)
		return
	}

	// Log successful completion
	middleware.Logger.WithFields(logrus.Fields{
		"request_id": requestID,
		"job_id":     jobID,
		"status":     jobStatus.Status,
	}).Info("Job status retrieved successfully")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(jobStatus)
}
