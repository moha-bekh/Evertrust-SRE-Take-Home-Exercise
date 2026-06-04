package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"certificate-inspector/internal/job"
	"certificate-inspector/internal/store"
)

func TestProcessorRetriesTransientFailureAndSucceeds(t *testing.T) {
	store := newWorkerFakeStore(job.Job{
		ID:          "job-1",
		Hostname:    "example.com",
		Port:        443,
		Status:      job.StatusPending,
		MaxAttempts: 2,
	})
	inspector := &fakeInspector{
		results: []inspectResult{
			{err: errors.New("timeout while dialing")},
			{result: job.CertificateResult{Hostname: "example.com", Port: 443}},
		},
	}
	processor := NewProcessor(store, NewQueue(10), inspector, slog.New(slog.NewTextHandler(io.Discard, nil)))

	processor.process(context.Background(), "job-1")

	got, err := store.GetJob(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if got.Status != job.StatusSucceeded {
		t.Fatalf("status = %q, want %q", got.Status, job.StatusSucceeded)
	}
	if got.Attempts != 2 {
		t.Fatalf("attempts = %d, want 2", got.Attempts)
	}
}

func TestProcessorMarksNonRetryableFailure(t *testing.T) {
	store := newWorkerFakeStore(job.Job{
		ID:          "job-1",
		Hostname:    "bad.example",
		Port:        443,
		Status:      job.StatusPending,
		MaxAttempts: 2,
	})
	inspector := &fakeInspector{
		results: []inspectResult{{err: errors.New("certificate verification failed")}},
	}
	processor := NewProcessor(store, NewQueue(10), inspector, slog.New(slog.NewTextHandler(io.Discard, nil)))

	processor.process(context.Background(), "job-1")

	got, err := store.GetJob(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if got.Status != job.StatusFailed {
		t.Fatalf("status = %q, want %q", got.Status, job.StatusFailed)
	}
	if got.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1", got.Attempts)
	}
}

type inspectResult struct {
	result job.CertificateResult
	err    error
}

type fakeInspector struct {
	mu      sync.Mutex
	results []inspectResult
}

func (i *fakeInspector) Inspect(context.Context, string, int) (job.CertificateResult, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if len(i.results) == 0 {
		return job.CertificateResult{}, errors.New("unexpected inspection call")
	}
	next := i.results[0]
	i.results = i.results[1:]
	return next.result, next.err
}

type workerFakeStore struct {
	mu  sync.Mutex
	job job.Job
}

func newWorkerFakeStore(item job.Job) *workerFakeStore {
	now := time.Now().UTC()
	item.CreatedAt = now
	item.UpdatedAt = now
	return &workerFakeStore{job: item}
}

func (s *workerFakeStore) CreateJob(context.Context, job.Job) (job.Job, error) {
	return job.Job{}, errors.New("not implemented")
}

func (s *workerFakeStore) GetJob(_ context.Context, id string) (job.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.job.ID != id {
		return job.Job{}, store.ErrNotFound
	}
	return s.job, nil
}

func (s *workerFakeStore) GetJobByIdempotencyKey(context.Context, string) (job.Job, error) {
	return job.Job{}, errors.New("not implemented")
}

func (s *workerFakeStore) ListRecentJobs(context.Context, int) ([]job.Job, error) {
	return nil, errors.New("not implemented")
}

func (s *workerFakeStore) MarkRunning(_ context.Context, id string) (job.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.job.ID != id {
		return job.Job{}, store.ErrNotFound
	}
	now := time.Now().UTC()
	s.job.Status = job.StatusRunning
	s.job.Attempts++
	s.job.UpdatedAt = now
	if s.job.StartedAt == nil {
		s.job.StartedAt = &now
	}
	return s.job, nil
}

func (s *workerFakeStore) MarkSucceeded(_ context.Context, id string, resultJSON string) (job.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.job.ID != id {
		return job.Job{}, store.ErrNotFound
	}
	now := time.Now().UTC()
	s.job.Status = job.StatusSucceeded
	s.job.ResultJSON = resultJSON
	s.job.UpdatedAt = now
	s.job.FinishedAt = &now
	return s.job, nil
}

func (s *workerFakeStore) MarkFailed(_ context.Context, id string, message string) (job.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.job.ID != id {
		return job.Job{}, store.ErrNotFound
	}
	now := time.Now().UTC()
	s.job.Status = job.StatusFailed
	s.job.Error = message
	s.job.UpdatedAt = now
	s.job.FinishedAt = &now
	return s.job, nil
}

func (s *workerFakeStore) Ping(context.Context) error {
	return nil
}

func (s *workerFakeStore) Close() error {
	return nil
}
