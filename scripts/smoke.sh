#!/usr/bin/env bash
set -euo pipefail

base_url="${BASE_URL:-http://localhost:8080}"

curl --fail --silent --show-error "${base_url}/livez" >/dev/null
curl --fail --silent --show-error "${base_url}/readyz" >/dev/null
curl --fail --silent --show-error "${base_url}/metrics" \
  | grep -E '^(http_requests_total|http_request_duration_seconds|http_requests_in_progress|tickets_created_total)'

echo "Ticketing API and application metrics are available"
