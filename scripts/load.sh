#!/usr/bin/env bash
set -euo pipefail

base_url="${BASE_URL:-http://localhost:8080}"
duration="${DURATION:-120}"
concurrency="${CONCURRENCY:-8}"

if ! [[ "$duration" =~ ^[0-9]+$ ]] || ! [[ "$concurrency" =~ ^[0-9]+$ ]]; then
  echo "DURATION and CONCURRENCY must be positive integers" >&2
  exit 1
fi

deadline=$((SECONDS + duration))

worker() {
  local worker_id="$1"
  local request_number=0
  while (( SECONDS < deadline )); do
    request_number=$((request_number + 1))
    if (( request_number % 5 == 0 )); then
      curl --silent --show-error --max-time 5 \
        --output /dev/null \
        --request POST "${base_url}/tickets" \
        --header 'Content-Type: application/json' \
        --data "{\"title\":\"load-${worker_id}-${request_number}\",\"priority\":\"medium\",\"reporter\":\"lab5\"}" \
        || true
    else
      curl --silent --show-error --max-time 5 \
        --output /dev/null "${base_url}/tickets" \
        || true
    fi
  done
}

echo "Generating traffic for ${duration}s with ${concurrency} workers against ${base_url}"
for worker_id in $(seq 1 "$concurrency"); do
  worker "$worker_id" &
done
wait
echo "Traffic generation finished"
