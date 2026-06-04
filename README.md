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
- health and readiness probes

## Requirements

- Docker
- Docker Compose
- Go 1.22, optional for local development

## Run

```bash
docker compose up --build
```

The API listens on `http://localhost:8080`.

## Exercise the API

```bash
./scripts/demo.sh
```

## API Examples

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

Health probes:

```txt
http://localhost:8080/healthz
http://localhost:8080/readyz
```

## Tests

```bash
make test
```

## Operational Notes

`/healthz` confirms the process is alive. `/readyz` checks SQLite connectivity and should be used before routing traffic to the service.

Logs are emitted as structured JSON to stdout. Metrics are exposed in Prometheus format at `/metrics`.

The current worker uses a bounded in-memory queue. This keeps local operation simple, but queued jobs are lost if the process exits before they are processed.

## Known Tradeoffs

- The queue is in-memory and not durable across restarts.
- SQLite is appropriate for local development, not production-scale multi-writer workloads.
- The current structure uses one worker for clarity.
- Authentication is intentionally out of scope.
- Distributed tracing and Kubernetes manifests are intentionally out of scope.
- TLS verification is enabled by default; internal self-signed certificates would need an explicit future option.

## What I Would Improve With 2-4 More Hours

- DB-backed queue or polling to recover pending jobs after restarts.
- Multiple workers with concurrency controls.
- More complete retry classification for transient network failures.
- API, store, and worker tests for the main paths.
- OpenTelemetry tracing.
- Grafana dashboard and alert rules.
