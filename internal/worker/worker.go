package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"certificate-inspector/internal/store"
)

type Queue struct {
	jobs chan string
}

func NewQueue(size int) *Queue {
	return &Queue{jobs: make(chan string, size)}
}

func (q *Queue) Enqueue(ctx context.Context, jobID string) error {
	select {
	case q.jobs <- jobID:
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
	inspector *CertificateInspector
	logger    *slog.Logger
	done      chan struct{}
	once      sync.Once
}

func NewProcessor(store store.Store, queue *Queue, inspector *CertificateInspector, logger *slog.Logger) *Processor {
	return &Processor{
		store:     store,
		queue:     queue,
		inspector: inspector,
		logger:    logger,
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
	item, err := p.store.MarkRunning(ctx, jobID)
	if err != nil {
		p.logger.Error("mark job running", slog.String("job_id", jobID), slog.String("error", err.Error()))
		return
	}

	result, err := p.inspector.Inspect(ctx, item.Hostname, item.Port)
	if err != nil {
		p.logger.Error("certificate inspection failed", slog.String("job_id", jobID), slog.String("hostname", item.Hostname), slog.String("error", err.Error()))
		_, _ = p.store.MarkFailed(ctx, jobID, err.Error())
		return
	}

	payload, err := json.Marshal(result)
	if err != nil {
		_, _ = p.store.MarkFailed(ctx, jobID, err.Error())
		return
	}

	if _, err := p.store.MarkSucceeded(ctx, jobID, string(payload)); err != nil {
		p.logger.Error("mark job succeeded", slog.String("job_id", jobID), slog.String("error", err.Error()))
	}
}
