#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
HOSTNAME="${HOSTNAME:-example.com}"
PORT="${PORT:-443}"
IDEMPOTENCY_KEY="${IDEMPOTENCY_KEY:-api-$(date +%s)-$$}"

echo "Submitting ${HOSTNAME}:${PORT}"
create_response="$(curl -fsS -X POST "${BASE_URL}/jobs" \
  -H 'Content-Type: application/json' \
  -d "{\"hostname\":\"${HOSTNAME}\",\"port\":${PORT},\"idempotency_key\":\"${IDEMPOTENCY_KEY}\"}")"
printf '%s\n' "${create_response}" | jq -C .

job_id="$(printf '%s' "${create_response}" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')"
if [ -z "${job_id}" ]; then
  echo "Could not parse job id" >&2
  exit 1
fi

echo
echo "Polling ${job_id}"
status="pending"
for _ in $(seq 1 30); do
  status_response="$(curl -fsS "${BASE_URL}/jobs/${job_id}/status")"
  printf '%s\n' "${status_response}" | jq -C .
  status="$(printf '%s' "${status_response}" | sed -n 's/.*"status":"\([^"]*\)".*/\1/p')"
  if [ "${status}" = "succeeded" ] || [ "${status}" = "failed" ]; then
    break
  fi
  sleep 1
done

echo
echo "Result"
curl -sS "${BASE_URL}/jobs/${job_id}/result" | jq -C .
echo

if [ "${status}" != "succeeded" ] && [ "${status}" != "failed" ]; then
  echo "Job did not complete before polling timeout" >&2
  exit 1
fi
