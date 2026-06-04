package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"certificate-inspector/internal/job"
	"certificate-inspector/internal/store"
	"certificate-inspector/internal/worker"
	"github.com/google/uuid"
)

type Dependencies struct {
	Store  store.Store
	Queue  *worker.Queue
	Logger *slog.Logger
}

type Handler struct {
	store  store.Store
	queue  *worker.Queue
	logger *slog.Logger
}

func NewHandler(deps Dependencies) *Handler {
	return &Handler{
		store:  deps.Store,
		queue:  deps.Queue,
		logger: deps.Logger,
	}
}

func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	var req createJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.Hostname = strings.TrimSpace(strings.ToLower(req.Hostname))
	if err := validateCreateJob(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Port == 0 {
		req.Port = 443
	}

	if req.IdempotencyKey != "" {
		existing, err := h.store.GetJobByIdempotencyKey(r.Context(), req.IdempotencyKey)
		if err == nil {
			writeJSON(w, http.StatusAccepted, responseFromJob(existing))
			return
		}
		if !errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusInternalServerError, "store lookup failed")
			return
		}
	}

	item, err := h.store.CreateJob(r.Context(), job.Job{
		ID:             uuid.NewString(),
		Hostname:       req.Hostname,
		Port:           req.Port,
		Status:         job.StatusPending,
		MaxAttempts:    2,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "job creation failed")
		return
	}

	if err := h.queue.Enqueue(r.Context(), item.ID); err != nil {
		writeError(w, http.StatusServiceUnavailable, "queue is unavailable")
		return
	}

	h.logger.Info("job submitted", slog.String("job_id", item.ID), slog.String("hostname", item.Hostname))
	writeJSON(w, http.StatusAccepted, responseFromJob(item))
}

func (h *Handler) getStatus(w http.ResponseWriter, r *http.Request) {
	item, err := h.getJobByPathID(r.Context(), r)
	if err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, statusResponse{
		ID:        item.ID,
		Status:    item.Status,
		Attempts:  item.Attempts,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	})
}

func (h *Handler) getResult(w http.ResponseWriter, r *http.Request) {
	item, err := h.getJobByPathID(r.Context(), r)
	if err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	if item.Status != job.StatusSucceeded && item.Status != job.StatusFailed {
		writeError(w, http.StatusConflict, "job has not completed")
		return
	}
	if item.Status == job.StatusFailed {
		writeError(w, http.StatusOK, item.Error)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(item.ResultJSON))
}

func (h *Handler) listJobs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.store.ListRecentJobs(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list jobs failed")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) readyz(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *Handler) getJobByPathID(ctx context.Context, r *http.Request) (job.Job, error) {
	id := r.PathValue("id")
	if id == "" {
		return job.Job{}, store.ErrNotFound
	}
	return h.store.GetJob(ctx, id)
}
