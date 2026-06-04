package store

import (
	"context"
	"testing"

	"certificate-inspector/internal/job"
)

func TestSQLiteStoreCreateGetAndIdempotency(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	created, err := store.CreateJob(ctx, job.Job{
		ID:             "job-1",
		Hostname:       "example.com",
		Port:           443,
		IdempotencyKey: "client-key",
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	if created.Status != job.StatusPending {
		t.Fatalf("status = %q, want %q", created.Status, job.StatusPending)
	}

	got, err := store.GetJob(ctx, "job-1")
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if got.Hostname != "example.com" || got.Port != 443 {
		t.Fatalf("job = %+v, want example.com:443", got)
	}

	byKey, err := store.GetJobByIdempotencyKey(ctx, "client-key")
	if err != nil {
		t.Fatalf("GetJobByIdempotencyKey() error = %v", err)
	}
	if byKey.ID != created.ID {
		t.Fatalf("idempotency lookup id = %q, want %q", byKey.ID, created.ID)
	}

	if _, err := store.CreateJob(ctx, job.Job{
		ID:             "job-2",
		Hostname:       "example.org",
		Port:           443,
		IdempotencyKey: "client-key",
	}); err == nil {
		t.Fatal("CreateJob() duplicate idempotency key error = nil")
	}
}

func TestSQLiteStoreListRecentJobs(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	for _, id := range []string{"job-1", "job-2", "job-3"} {
		if _, err := store.CreateJob(ctx, job.Job{ID: id, Hostname: id + ".example.com", Port: 443}); err != nil {
			t.Fatalf("CreateJob(%s) error = %v", id, err)
		}
	}

	jobs, err := store.ListRecentJobs(ctx, 2)
	if err != nil {
		t.Fatalf("ListRecentJobs() error = %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("len(jobs) = %d, want 2", len(jobs))
	}
	if jobs[0].ID == jobs[1].ID {
		t.Fatalf("expected distinct jobs, got %+v", jobs)
	}
}

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()

	store, err := OpenSQLite(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	_, err = store.db.Exec(`
CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    hostname TEXT NOT NULL,
    port INTEGER NOT NULL,
    status TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 2,
    error TEXT,
    result_json TEXT,
    idempotency_key TEXT UNIQUE,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    started_at TEXT,
    finished_at TEXT
)`)
	if err != nil {
		t.Fatalf("create schema error = %v", err)
	}
	return store
}
