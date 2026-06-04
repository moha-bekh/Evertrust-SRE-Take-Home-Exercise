package api

import (
	"net/http"

	"certificate-inspector/internal/observability"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewRouter(deps Dependencies) http.Handler {
	handler := NewHandler(deps)
	mux := http.NewServeMux()

	mux.HandleFunc("POST /jobs", handler.createJob)
	mux.HandleFunc("GET /jobs", handler.listJobs)
	mux.HandleFunc("GET /jobs/{id}/status", handler.getStatus)
	mux.HandleFunc("GET /jobs/{id}/result", handler.getResult)
	mux.HandleFunc("GET /healthz", handler.healthz)
	mux.HandleFunc("GET /readyz", handler.readyz)
	mux.Handle("GET /metrics", promhttp.Handler())

	return observability.HTTPMetricsMiddleware(mux)
}
