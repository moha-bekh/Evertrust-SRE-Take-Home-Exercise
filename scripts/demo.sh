#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
IDEMPOTENCY_KEY="demo-example-com"

echo "Submitting certificate inspection job"
CREATE_RESPONSE="$(curl -fsS -X POST "${BASE_URL}/jobs" \
  -H 'Content-Type: application/json' \
  -d "{\"hostname\":\"example.com\",\"port\":443,\"idempotency_key\":\"${IDEMPOTENCY_KEY}\"}")"
echo "${CREATE_RESPONSE}"

JOB_ID="$(printf '%s' "${CREATE_RESPONSE}" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')"
if [ -z "${JOB_ID}" ]; then
  echo "Could not parse job id" >&2
  exit 1
fi

echo
echo "Polling job status"
for _ in $(seq 1 20); do
  STATUS_RESPONSE="$(curl -fsS "${BASE_URL}/jobs/${JOB_ID}/status")"
  echo "${STATUS_RESPONSE}"
  STATUS="$(printf '%s' "${STATUS_RESPONSE}" | sed -n 's/.*"status":"\([^"]*\)".*/\1/p')"
  if [ "${STATUS}" = "succeeded" ] || [ "${STATUS}" = "failed" ]; then
    break
  fi
  sleep 1
done

echo
echo "Fetching result"
curl -fsS "${BASE_URL}/jobs/${JOB_ID}/result" || true
echo

echo
echo "Submitting invalid hostname"
curl -fsS -X POST "${BASE_URL}/jobs" \
  -H 'Content-Type: application/json' \
  -d '{"hostname":"https://bad.example","port":443}' || true
echo

echo
echo "Submitting duplicate idempotency key"
curl -fsS -X POST "${BASE_URL}/jobs" \
  -H 'Content-Type: application/json' \
  -d "{\"hostname\":\"example.com\",\"port\":443,\"idempotency_key\":\"${IDEMPOTENCY_KEY}\"}"
echo

echo
echo "Metrics are available at ${BASE_URL}/metrics"
