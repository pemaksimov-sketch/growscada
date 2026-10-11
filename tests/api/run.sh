#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
mkdir -p artifacts/api
export API_TEST_IMAGE_TAG="run-$$"
compose=(docker compose -p "growscada-api-$$" -f tests/api/compose.yaml)
cleanup() {
  result=$?
  "${compose[@]}" logs --no-color > artifacts/api/containers.log 2>&1 || true
  "${compose[@]}" down --volumes --remove-orphans || true
  exit "$result"
}
trap cleanup EXIT
"${compose[@]}" build
"${compose[@]}" up --detach --wait --wait-timeout 120 server
"${compose[@]}" run --rm bruno
