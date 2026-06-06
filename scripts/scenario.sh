#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
SCENARIO="${1:-}"

json_field() {
  jq -r "$1"
}

submit_job() {
  local hostname="$1"
  local port="${2:-443}"
  local key="${3:-scenario-$(date +%s)-$$}"

  curl -fsS -X POST "${BASE_URL}/jobs" \
    -H 'Content-Type: application/json' \
    -d "{\"hostname\":\"${hostname}\",\"port\":${port},\"idempotency_key\":\"${key}\"}"
}

poll_until_terminal() {
  local job_id="$1"
  local expected_status="${2:-}"
  local status="pending"
  local response=""

  for _ in $(seq 1 30); do
    response="$(curl -fsS "${BASE_URL}/jobs/${job_id}/status")"
    status="$(printf '%s' "${response}" | json_field '.status')"
    printf '%s\n' "${response}" | jq -C .

    if [ "${status}" = "succeeded" ] || [ "${status}" = "failed" ]; then
      break
    fi
    sleep 1
  done

  if [ -n "${expected_status}" ] && [ "${status}" != "${expected_status}" ]; then
    echo "Expected status ${expected_status}, got ${status}" >&2
    exit 1
  fi
}

case "${SCENARIO}" in
  validation)
    echo "Scenario: invalid hostname is rejected with HTTP 400"
    status_code="$(curl -sS -o /tmp/certificate-inspector-validation.json -w '%{http_code}' \
      -X POST "${BASE_URL}/jobs" \
      -H 'Content-Type: application/json' \
      -d '{"hostname":"https://bad.example","port":443}')"
    jq -C . /tmp/certificate-inspector-validation.json
    test "${status_code}" = "400"
    ;;

  idempotency)
    echo "Scenario: duplicate idempotency key returns the existing job"
    key="scenario-idempotency-$(date +%s)-$$"
    first="$(submit_job "example.com" 443 "${key}")"
    second="$(submit_job "example.com" 443 "${key}")"
    first_id="$(printf '%s' "${first}" | json_field '.id')"
    second_id="$(printf '%s' "${second}" | json_field '.id')"
    printf '%s\n' "${first}" | jq -C .
    printf '%s\n' "${second}" | jq -C .
    test "${first_id}" = "${second_id}"
    ;;

  result-before-completion)
    echo "Scenario: result endpoint returns HTTP 409 while the job is still pending/running"
    response="$(submit_job "example.com" 443 "scenario-conflict-$(date +%s)-$$")"
    job_id="$(printf '%s' "${response}" | json_field '.id')"
    printf '%s\n' "${response}" | jq -C .
    status_code="$(curl -sS -o /tmp/certificate-inspector-result-before-completion.json -w '%{http_code}' \
      "${BASE_URL}/jobs/${job_id}/result")"
    jq -C . /tmp/certificate-inspector-result-before-completion.json
    test "${status_code}" = "409"
    ;;

  network-failure)
    echo "Scenario: DNS/network failure is persisted as a failed job"
    hostname="${SCENARIO_NETWORK_HOSTNAME:-does-not-exist.invalid}"
    port="${SCENARIO_NETWORK_PORT:-443}"
    response="$(submit_job "${hostname}" "${port}" "scenario-network-failure-$(date +%s)-$$")"
    job_id="$(printf '%s' "${response}" | json_field '.id')"
    printf '%s\n' "${response}" | jq -C .
    poll_until_terminal "${job_id}" "failed"
    curl -fsS "${BASE_URL}/jobs/${job_id}/result" | jq -C .
    ;;

  timeout)
    echo "Scenario: connection timeout is retried and eventually marked failed"
    hostname="${SCENARIO_TIMEOUT_HOSTNAME:-example.com}"
    port="${SCENARIO_TIMEOUT_PORT:-1}"
    response="$(submit_job "${hostname}" "${port}" "scenario-timeout-$(date +%s)-$$")"
    job_id="$(printf '%s' "${response}" | json_field '.id')"
    printf '%s\n' "${response}" | jq -C .
    poll_until_terminal "${job_id}" "failed"
    curl -fsS "${BASE_URL}/jobs/${job_id}/status" | jq -C .
    curl -fsS "${BASE_URL}/jobs/${job_id}/result" | jq -C .
    ;;

  *)
    echo "Usage: $0 validation|idempotency|result-before-completion|network-failure|timeout" >&2
    exit 1
    ;;
esac
