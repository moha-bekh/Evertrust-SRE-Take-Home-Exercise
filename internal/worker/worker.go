package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"

	"certificate-inspector/internal/job"
	"certificate-inspector/internal/observability"
	"certificate-inspector/internal/store"
)

type Inspector interface {
	Inspect(ctx context.Context, hostname string, port int) (job.CertificateResult, error)
}

type Queue struct {
	jobs chan string
}

func NewQueue(size int) *Queue {
	return &Queue{jobs: make(chan string, size)}
}

func (q *Queue) Enqueue(ctx context.Context, jobID string) error {
	select {
	case q.jobs <- jobID:
		observability.JobQueueDepth.Set(float64(q.Depth()))
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *Queue) Depth() int {
	return len(q.jobs)
}

type Processor struct {
	store     store.Store
	queue     *Queue
	inspector Inspector
	logger    *slog.Logger
	jobDelay  time.Duration
	done      chan struct{}
	once      sync.Once
}

func NewProcessor(store store.Store, queue *Queue, inspector Inspector, logger *slog.Logger, jobDelay time.Duration) *Processor {
	return &Processor{
		store:     store,
		queue:     queue,
		inspector: inspector,
		logger:    logger,
		jobDelay:  jobDelay,
		done:      make(chan struct{}),
	}
}

func (p *Processor) Start(ctx context.Context) {
	go func() {
		defer close(p.done)
		for {
			select {
			case <-ctx.Done():
				return
			case jobID := <-p.queue.jobs:
				observability.JobQueueDepth.Set(float64(p.queue.Depth()))
				p.process(ctx, jobID)
			}
		}
	}()
}

func (p *Processor) Stop(ctx context.Context) {
	p.once.Do(func() {
		select {
		case <-p.done:
		case <-ctx.Done():
		}
	})
}

func (p *Processor) process(ctx context.Context, jobID string) {
	started := time.Now()
	observability.JobsInProgress.Inc()
	defer observability.JobsInProgress.Dec()

	for {
		item, err := p.store.MarkRunning(ctx, jobID)
		if err != nil {
			p.logger.Error("mark job running", slog.String("job_id", jobID), slog.String("error", err.Error()))
			return
		}

		if p.jobDelay > 0 {
			select {
			case <-time.After(p.jobDelay):
			case <-ctx.Done():
				return
			}
		}

		inspectionStarted := time.Now()
		result, err := p.inspector.Inspect(ctx, item.Hostname, item.Port)
		observability.CertificateInspectionDuration.Observe(time.Since(inspectionStarted).Seconds())
		if err != nil {
			p.logger.Error(
				"certificate inspection failed",
				slog.String("job_id", jobID),
				slog.String("hostname", item.Hostname),
				slog.Int("attempt", item.Attempts),
				slog.String("error", err.Error()),
			)

			if item.Attempts < item.MaxAttempts && retryable(err) {
				observability.JobsRetriedTotal.Inc()
				continue
			}

			_, _ = p.store.MarkFailed(ctx, jobID, err.Error())
			observability.JobsProcessedTotal.WithLabelValues(string(job.StatusFailed)).Inc()
			observability.CertificateInspectionTotal.WithLabelValues(string(job.StatusFailed)).Inc()
			observability.JobProcessingDuration.Observe(time.Since(started).Seconds())
			return
		}

		payload, err := json.Marshal(result)
		if err != nil {
			_, _ = p.store.MarkFailed(ctx, jobID, err.Error())
			observability.JobsProcessedTotal.WithLabelValues(string(job.StatusFailed)).Inc()
			observability.JobProcessingDuration.Observe(time.Since(started).Seconds())
			return
		}

		if _, err := p.store.MarkSucceeded(ctx, jobID, string(payload)); err != nil {
			p.logger.Error("mark job succeeded", slog.String("job_id", jobID), slog.String("error", err.Error()))
			return
		}

		p.logger.Info(
			"job completed",
			slog.String("job_id", jobID),
			slog.String("hostname", item.Hostname),
			slog.Int64("duration_ms", time.Since(started).Milliseconds()),
			slog.String("status", string(job.StatusSucceeded)),
		)
		observability.JobsProcessedTotal.WithLabelValues(string(job.StatusSucceeded)).Inc()
		observability.CertificateInspectionTotal.WithLabelValues(string(job.StatusSucceeded)).Inc()
		observability.JobProcessingDuration.Observe(time.Since(started).Seconds())
		return
	}
}

func retryable(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}

	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "timeout") ||
		strings.Contains(message, "connection reset") ||
		strings.Contains(message, "temporary")
}
