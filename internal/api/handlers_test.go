package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"certificate-inspector/internal/job"
	"certificate-inspector/internal/store"
	"certificate-inspector/internal/worker"
)

func TestCreateJobValidReturnsAccepted(t *testing.T) {
	fakeStore := newFakeStore()
	router := NewRouter(Dependencies{
		Store:  fakeStore,
		Queue:  worker.NewQueue(10),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	rec := request(router, http.MethodPost, "/jobs", `{"hostname":"example.com","port":443}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	if len(fakeStore.jobs) != 1 {
		t.Fatalf("jobs stored = %d, want 1", len(fakeStore.jobs))
	}
}

func TestCreateJobInvalidHostnameReturnsBadRequest(t *testing.T) {
	router := NewRouter(Dependencies{
		Store:  newFakeStore(),
		Queue:  worker.NewQueue(10),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	rec := request(router, http.MethodPost, "/jobs", `{"hostname":"https://bad.example","port":443}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateJobDuplicateIdempotencyKeyReturnsExistingJob(t *testing.T) {
	fakeStore := newFakeStore()
	existing, err := fakeStore.CreateJob(context.Background(), job.Job{
		ID:             "existing-job",
		Hostname:       "example.com",
		Port:           443,
		IdempotencyKey: "same-key",
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	router := NewRouter(Dependencies{
		Store:  fakeStore,
		Queue:  worker.NewQueue(10),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	rec := request(router, http.MethodPost, "/jobs", `{"hostname":"example.com","port":443,"idempotency_key":"same-key"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	var body createJobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body error = %v", err)
	}
	if body.ID != existing.ID {
		t.Fatalf("id = %q, want %q", body.ID, existing.ID)
	}
	if len(fakeStore.jobs) != 1 {
		t.Fatalf("jobs stored = %d, want 1", len(fakeStore.jobs))
	}
}

func TestMissingJobReturnsNotFound(t *testing.T) {
	router := NewRouter(Dependencies{
		Store:  newFakeStore(),
		Queue:  worker.NewQueue(10),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	rec := request(router, http.MethodGet, "/jobs/missing/status", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestResultBeforeCompletionReturnsConflict(t *testing.T) {
	fakeStore := newFakeStore()
	if _, err := fakeStore.CreateJob(context.Background(), job.Job{ID: "job-1", Hostname: "example.com", Port: 443}); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	router := NewRouter(Dependencies{
		Store:  fakeStore,
		Queue:  worker.NewQueue(10),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	rec := request(router, http.MethodGet, "/jobs/job-1/result", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func request(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

type fakeStore struct {
	mu   sync.Mutex
	jobs map[string]job.Job
}

func newFakeStore() *fakeStore {
	return &fakeStore{jobs: map[string]job.Job{}}
}

func (s *fakeStore) CreateJob(_ context.Context, candidate job.Job) (job.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	candidate.CreatedAt = now
	candidate.UpdatedAt = now
	if candidate.Status == "" {
		candidate.Status = job.StatusPending
	}
	if candidate.MaxAttempts == 0 {
		candidate.MaxAttempts = 2
	}
	if candidate.IdempotencyKey != "" {
		for _, existing := range s.jobs {
			if existing.IdempotencyKey == candidate.IdempotencyKey {
				return job.Job{}, errors.New("duplicate idempotency key")
			}
		}
	}
	s.jobs[candidate.ID] = candidate
	return candidate, nil
}

func (s *fakeStore) GetJob(_ context.Context, id string) (job.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.jobs[id]
	if !ok {
		return job.Job{}, store.ErrNotFound
	}
	return item, nil
}

func (s *fakeStore) GetJobByIdempotencyKey(_ context.Context, key string) (job.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, item := range s.jobs {
		if item.IdempotencyKey == key {
			return item, nil
		}
	}
	return job.Job{}, store.ErrNotFound
}

func (s *fakeStore) ListRecentJobs(_ context.Context, _ int) ([]job.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]job.Job, 0, len(s.jobs))
	for _, item := range s.jobs {
		items = append(items, item)
	}
	return items, nil
}

func (s *fakeStore) MarkRunning(_ context.Context, id string) (job.Job, error) {
	return s.GetJob(context.Background(), id)
}

func (s *fakeStore) MarkSucceeded(_ context.Context, id string, resultJSON string) (job.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item := s.jobs[id]
	item.Status = job.StatusSucceeded
	item.ResultJSON = resultJSON
	s.jobs[id] = item
	return item, nil
}

func (s *fakeStore) MarkFailed(_ context.Context, id string, message string) (job.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item := s.jobs[id]
	item.Status = job.StatusFailed
	item.Error = message
	s.jobs[id] = item
	return item, nil
}

func (s *fakeStore) Ping(context.Context) error {
	return nil
}

func (s *fakeStore) Close() error {
	return nil
}
