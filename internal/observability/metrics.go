package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	HTTPRequestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "http_requests_total", Help: "Total HTTP requests."},
		[]string{"method", "path", "status"},
	)
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "HTTP request duration."},
		[]string{"method", "path"},
	)
	JobsSubmittedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "jobs_submitted_total", Help: "Total submitted jobs."},
	)
	JobsProcessedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "jobs_processed_total", Help: "Total processed jobs."},
		[]string{"status"},
	)
	JobsInProgress = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "jobs_in_progress", Help: "Jobs currently being processed."},
	)
	JobsRetriedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{Name: "jobs_retried_total", Help: "Total job retries."},
	)
	JobProcessingDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{Name: "job_processing_duration_seconds", Help: "Job processing duration."},
	)
	JobQueueDepth = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "job_queue_depth", Help: "Current in-memory job queue depth."},
	)
	CertificateInspectionTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "certificate_inspection_total", Help: "Total certificate inspections."},
		[]string{"status"},
	)
	CertificateInspectionDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{Name: "certificate_inspection_duration_seconds", Help: "Certificate inspection duration."},
	)
)

func init() {
	prometheus.MustRegister(
		HTTPRequestTotal,
		HTTPRequestDuration,
		JobsSubmittedTotal,
		JobsProcessedTotal,
		JobsInProgress,
		JobsRetriedTotal,
		JobProcessingDuration,
		JobQueueDepth,
		CertificateInspectionTotal,
		CertificateInspectionDuration,
	)
}

func HTTPMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		status := strconv.Itoa(recorder.status)
		path := r.Pattern
		if path == "" {
			path = r.URL.Path
		}
		HTTPRequestTotal.WithLabelValues(r.Method, path, status).Inc()
		HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(time.Since(started).Seconds())
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
