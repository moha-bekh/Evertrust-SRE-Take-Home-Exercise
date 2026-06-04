# Architecture Note

## Overview

The system accepts certificate inspection jobs over HTTP, persists them in SQLite, enqueues them in an in-memory queue, and processes them asynchronously with a background worker.

## Data Flow

1. Client submits a hostname.
2. API validates input.
3. Job is stored as `pending`.
4. Job ID is pushed to the queue.
5. Worker marks the job `running`.
6. Worker performs TLS inspection.
7. Result or failure is stored.
8. Client retrieves status and result over the API.

## Design Decisions

### Go

Go was chosen for a small static service with straightforward HTTP, TLS, signal handling, and concurrency support.

### SQLite

SQLite keeps local setup simple while still giving durable job metadata and results. The store interface keeps the persistence boundary explicit.

### In-Memory Queue

The in-memory queue keeps the implementation small and demonstrates async processing without a broker. The tradeoff is clear: queued-but-unprocessed jobs are lost on restart. A production version should use a durable broker or a DB-backed queue with polling and lease semantics.

### Certificate Inspector Domain

Certificate inspection is small enough to implement cleanly and relevant to PKI and certificate lifecycle operations. It also creates realistic failure modes: DNS issues, TLS verification errors, timeouts, and expiry checks.

### Observability

The service exposes structured logs, Prometheus metrics, health probes, and a provisioned Grafana dashboard. This gives operators answers to basic questions:

- Is the API receiving traffic?
- Is the database reachable?
- Are jobs being accepted?
- Are jobs completing or failing?
- How long do requests take?
- Is the worker queue building up?
- Are certificate inspections succeeding or failing?

Grafana is intentionally provisioned through files under `ops/grafana` so the local run experience is reproducible. The dashboard uses low-cardinality service metrics and avoids hostname labels.

## Reliability Behavior

Input validation rejects empty hostnames, hostnames with schemes, malformed DNS names, IP addresses, invalid ports, and overly long idempotency keys.

Idempotency is implemented with an optional `idempotency_key`. If the key already exists, the API returns the existing job and does not enqueue duplicate work.

Timeouts are configured for HTTP reads/writes and certificate inspection. TLS verification is enabled by default.

Graceful shutdown stops the HTTP server and lets the worker stop through context cancellation.

## Tradeoffs

- Local-first design favors easy review over production-grade durability.
- No external broker keeps setup small but leaves queue recovery incomplete.
- SQLite avoids service dependencies but is not a replacement for a production job database at scale.
- No auth is included because the exercise focuses on service behavior and operability.
- Certificate failure classification is currently coarse.
- Hostname labels are avoided in metrics to prevent high-cardinality series.
- Grafana anonymous access is enabled only to reduce friction for local review.

## Future Improvements

- DB-backed queue with recovery of pending/running jobs after restart.
- Multiple workers with bounded concurrency and backpressure.
- Retry only transient-looking network failures.
- More complete tests around API behavior, store behavior, and worker retries.
- OpenTelemetry traces and structured correlation IDs.
- Grafana and Prometheus alert rules.
- Explicit disabled-by-default option for internal/self-signed certificates.
