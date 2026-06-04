package store

import (
	"context"

	"certificate-inspector/internal/job"
)

type Store interface {
	CreateJob(ctx context.Context, candidate job.Job) (job.Job, error)
	GetJob(ctx context.Context, id string) (job.Job, error)
	GetJobByIdempotencyKey(ctx context.Context, key string) (job.Job, error)
	ListRecentJobs(ctx context.Context, limit int) ([]job.Job, error)
	MarkRunning(ctx context.Context, id string) (job.Job, error)
	MarkSucceeded(ctx context.Context, id string, resultJSON string) (job.Job, error)
	MarkFailed(ctx context.Context, id string, message string) (job.Job, error)
	Ping(ctx context.Context) error
	Close() error
}
