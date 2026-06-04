#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
HOSTNAMES="${HOSTNAMES:-example.com}"
RUN_ID="$(date +%s)-$$"

IFS='|' read -r -a HOSTNAME_LIST <<< "${HOSTNAMES}"

if [ "${#HOSTNAME_LIST[@]}" -eq 0 ]; then
  echo "No hostnames provided" >&2
  exit 1
fi

declare -a JOB_IDS=()
declare -a JOB_HOSTNAMES=()
FIRST_HOSTNAME=""
FIRST_IDEMPOTENCY_KEY=""

echo "Submitting certificate inspection jobs"
for raw_hostname in "${HOSTNAME_LIST[@]}"; do
  hostname="$(printf '%s' "${raw_hostname}" | xargs)"
  if [ -z "${hostname}" ]; then
    continue
  fi

  idempotency_key="demo-${RUN_ID}-${hostname//[^A-Za-z0-9]/-}"
  if [ -z "${FIRST_HOSTNAME}" ]; then
    FIRST_HOSTNAME="${hostname}"
    FIRST_IDEMPOTENCY_KEY="${idempotency_key}"
  fi

  create_response="$(curl -fsS -X POST "${BASE_URL}/jobs" \
    -H 'Content-Type: application/json' \
    -d "{\"hostname\":\"${hostname}\",\"port\":443,\"idempotency_key\":\"${idempotency_key}\"}")"
  echo "${create_response}"

  job_id="$(printf '%s' "${create_response}" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')"
  if [ -z "${job_id}" ]; then
    echo "Could not parse job id for ${hostname}" >&2
    exit 1
  fi

  JOB_IDS+=("${job_id}")
  JOB_HOSTNAMES+=("${hostname}")
done

if [ "${#JOB_IDS[@]}" -eq 0 ]; then
  echo "No valid hostnames submitted" >&2
  exit 1
fi

echo
echo "Polling job status"
declare -a JOB_DONE=()
for index in "${!JOB_IDS[@]}"; do
  JOB_DONE[index]=0
done

for _ in $(seq 1 30); do
  all_done=1
  for index in "${!JOB_IDS[@]}"; do
    if [ "${JOB_DONE[index]}" = "1" ]; then
      continue
    fi

    job_id="${JOB_IDS[index]}"
    hostname="${JOB_HOSTNAMES[index]}"
    status_response="$(curl -fsS "${BASE_URL}/jobs/${job_id}/status")"
    echo "${hostname}: ${status_response}"
    status="$(printf '%s' "${status_response}" | sed -n 's/.*"status":"\([^"]*\)".*/\1/p')"
    if [ "${status}" = "succeeded" ] || [ "${status}" = "failed" ]; then
      JOB_DONE[index]=1
    else
      all_done=0
    fi
  done

  if [ "${all_done}" = "1" ]; then
    break
  fi
  sleep 1
done

echo
echo "Fetching results"
for index in "${!JOB_IDS[@]}"; do
  job_id="${JOB_IDS[index]}"
  hostname="${JOB_HOSTNAMES[index]}"
  echo "${hostname}:"
  curl -sS "${BASE_URL}/jobs/${job_id}/result" || true
  echo
done

echo
echo "Submitting invalid hostname"
curl -sS -X POST "${BASE_URL}/jobs" \
  -H 'Content-Type: application/json' \
  -d '{"hostname":"https://bad.example","port":443}' || true
echo

echo
echo "Submitting duplicate idempotency key"
curl -fsS -X POST "${BASE_URL}/jobs" \
  -H 'Content-Type: application/json' \
  -d "{\"hostname\":\"${FIRST_HOSTNAME}\",\"port\":443,\"idempotency_key\":\"${FIRST_IDEMPOTENCY_KEY}\"}"
echo

echo
echo "Metrics are available at ${BASE_URL}/metrics"
