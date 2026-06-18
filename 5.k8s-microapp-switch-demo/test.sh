#!/bin/bash
set -euo pipefail

PORT=18080

cleanup() {
  kill "$PF_PID" 2>/dev/null || true
}
trap cleanup EXIT

kubectl port-forward svc/nginx-gateway-svc "${PORT}:80" >/dev/null 2>&1 &
PF_PID=$!

for _ in $(seq 1 20); do
  if curl -s --connect-timeout 1 "http://127.0.0.1:${PORT}/" >/dev/null 2>&1; then
    break
  fi
  sleep 0.3
done

for i in $(seq 1 30); do
  curl -s --max-time 5 "http://127.0.0.1:${PORT}/"
  echo
done
