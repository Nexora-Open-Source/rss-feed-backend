// Package monitoring provides Prometheus metrics endpoint
package monitoring

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsHandler returns an HTTP handler for serving Prometheus metrics
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// SetupMetricsEndpoint configures the metrics endpoint on the given router
func SetupMetricsEndpoint(router *mux.Router) {
	router.Handle("/metrics", MetricsHandler()).Methods("GET")
}
