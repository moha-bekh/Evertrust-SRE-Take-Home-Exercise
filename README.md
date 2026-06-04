# Certificate Inspection Service

## Overview

Small asynchronous service that inspects TLS certificates for submitted hostnames.

The service accepts a hostname over HTTP, persists a job in SQLite, pushes the job ID to an in-memory queue, and processes the certificate inspection in a background worker.

## Why This Service?

This project demonstrates a production-style SRE workflow:

- asynchronous processing
- persistent job tracking
- retry and timeout behavior
- structured JSON logs
- Prometheus metrics
- provisioned Grafana dashboard
- health and readiness probes

## Requirements

- Docker
- Docker Compose
- Task
- Go 1.22, optional for local development

## Run

```bash
task docker-up
```

The API listens on `http://localhost:8080`.

The Docker Compose stack sets a small `WORKER_JOB_DELAY` to make queue and in-progress metrics visible during local demos. The default outside Compose is no artificial delay.

## Exercise the API

```bash
task demo
```

Run a batch demo to make queue and worker behavior more visible:

```bash
task demo-batch
```

Run a larger batch to make transient queue and in-progress metrics easier to catch in Grafana:

```bash
task demo-big-batch
```

Reset the Docker SQLite volume before a clean demo run:

```bash
task db-reset
task docker-up-detached
task demo-big-batch
```

Or provide a custom hostname list:

```bash
HOSTNAMES="example.com|github.com|expired.badssl.com" task demo
```

## API Examples

Use Task shortcuts:

```bash
task api:health
task api:ready
task api:submit HOSTNAME=example.com
task api:inspect HOSTNAME=example.com
task api:list LIMIT=10
task api:status JOB_ID=<job-id>
task api:result JOB_ID=<job-id>
```

Submit a job:

```bash
curl -X POST http://localhost:8080/jobs \
  -H 'Content-Type: application/json' \
  -d '{"hostname":"example.com","port":443,"idempotency_key":"example-com"}'
```

Check status:

```bash
curl http://localhost:8080/jobs/<job-id>/status
```

Get result:

```bash
curl http://localhost:8080/jobs/<job-id>/result
```

List recent jobs:

```bash
curl 'http://localhost:8080/jobs?limit=10'
```

## Observability

Metrics:

```txt
http://localhost:8080/metrics
```

Prometheus:

```txt
http://localhost:9090
```

Grafana dashboard:

```txt
http://localhost:3000/d/certificate-inspector-overview/certificate-inspector-overview
```

Health probes:

```txt
http://localhost:8080/healthz
http://localhost:8080/readyz
```

## Tests

```bash
task test
```

## Operational Notes

`/healthz` confirms the process is alive. `/readyz` checks SQLite connectivity and should be used before routing traffic to the service.

Logs are emitted as structured JSON to stdout. Metrics are exposed in Prometheus format at `/metrics`.

Grafana is provisioned automatically with a Prometheus datasource and a `Certificate Inspector Overview` dashboard. Anonymous viewer access is enabled for local review only.

`jobs_in_progress` and `job_queue_depth` are short-lived gauges. The local stack uses a 1-second Prometheus scrape interval, a 5-second Grafana refresh interval, and the dashboard displays their 5-minute maximum to make brief queue activity visible during demos.

The current worker uses a bounded in-memory queue. This keeps local operation simple, but queued jobs are lost if the process exits before they are processed.

## Known Tradeoffs

- The queue is in-memory and not durable across restarts.
- SQLite is appropriate for local development, not production-scale multi-writer workloads.
- The current structure uses one worker for clarity.
- Authentication is intentionally out of scope.
- Distributed tracing and Kubernetes manifests are intentionally out of scope.
- TLS verification is enabled by default; internal self-signed certificates would need an explicit future option.
- Grafana is configured for local review convenience, not hardened production access.

## What I Would Improve With 2-4 More Hours

- DB-backed queue or polling to recover pending jobs after restarts.
- Multiple workers with concurrency controls.
- More complete retry classification for transient network failures.
- API, store, and worker tests for the main paths.
- OpenTelemetry tracing.
- Grafana alert rules.
