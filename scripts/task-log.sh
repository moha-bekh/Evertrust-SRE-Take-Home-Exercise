#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 3 ] || [ "${2}" != "--" ]; then
  echo "Usage: $0 <task-name> -- <command> [args...]" >&2
  exit 2
fi

task_name="$1"
shift 2

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
safe_task_name="$(printf '%s' "${task_name}" | tr -c 'A-Za-z0-9._-' '-')"
log_root="${TASK_LOG_DIR:-logs}"
run_dir="${log_root}/${timestamp}-${safe_task_name}-$$"
output_log="${run_dir}/output.log"
output_jsonl="${run_dir}/output.jsonl"
metadata_json="${run_dir}/metadata.json"

mkdir -p "${run_dir}"
touch "${output_log}" "${output_jsonl}"

command_string="$(printf '%q ' "$@")"
started_epoch="$(date +%s)"
started_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

jq -n \
  --arg task "${task_name}" \
  --arg command "${command_string% }" \
  --arg started_at "${started_at}" \
  --arg output_log "${output_log}" \
  --arg output_jsonl "${output_jsonl}" \
  '{
    task: $task,
    command: $command,
    started_at: $started_at,
    status: "running",
    output_log: $output_log,
    output_jsonl: $output_jsonl
  }' > "${metadata_json}"

set +e
"$@" 2>&1 | while IFS= read -r line || [ -n "${line}" ]; do
  clean_line="$(printf '%s' "${line}" | perl -pe 's/\e\[[0-?]*[ -\/]*[@-~]//g')"
  printf '%s\n' "${line}"
  jq -nc \
    --arg timestamp "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --arg task "${task_name}" \
    --arg line "${clean_line}" \
    '{timestamp: $timestamp, task: $task, stream: "combined", line: $line}' >> "${output_jsonl}"
  printf '%s\n' "${clean_line}" >> "${output_log}"
done
exit_code="${PIPESTATUS[0]}"
set -e

finished_epoch="$(date +%s)"
finished_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
duration_seconds="$((finished_epoch - started_epoch))"
status="succeeded"
if [ "${exit_code}" -ne 0 ]; then
  status="failed"
fi

jq -n \
  --arg task "${task_name}" \
  --arg command "${command_string% }" \
  --arg started_at "${started_at}" \
  --arg finished_at "${finished_at}" \
  --arg status "${status}" \
  --arg output_log "${output_log}" \
  --arg output_jsonl "${output_jsonl}" \
  --argjson exit_code "${exit_code}" \
  --argjson duration_seconds "${duration_seconds}" \
  '{
    task: $task,
    command: $command,
    started_at: $started_at,
    finished_at: $finished_at,
    duration_seconds: $duration_seconds,
    exit_code: $exit_code,
    status: $status,
    output_log: $output_log,
    output_jsonl: $output_jsonl
  }' > "${metadata_json}"

echo "Task log written to ${run_dir}"
exit "${exit_code}"
