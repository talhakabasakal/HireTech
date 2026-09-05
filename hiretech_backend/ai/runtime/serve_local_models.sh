#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PYTHON_BIN="${PYTHON_BIN:-python3}"
ADAPTER_ROOT="${AI_ADAPTER_ROOT:-$ROOT/adapters}"
LOG_ROOT="${AI_LOG_ROOT:-${TMPDIR:-/tmp}/hiretech-ai}"

if [[ ! -d "$ADAPTER_ROOT/interviewer" || ! -d "$ADAPTER_ROOT/evaluator" ]]; then
  echo "QLoRA adapterları bulunamadı: $ADAPTER_ROOT/{interviewer,evaluator}" >&2
  exit 1
fi

export PYTHONPATH="$ROOT${PYTHONPATH:+:$PYTHONPATH}"
mkdir -p "$LOG_ROOT"

run_role() {
  local role="$1"
  local port="$2"
  local api_key=""
  if [[ "$role" == "interviewer" ]]; then
    api_key="${AI_INTERVIEWER_API_KEY:-}"
  else
    api_key="${AI_EVALUATOR_API_KEY:-}"
  fi
  AI_ROLE="$role" AI_PORT="$port" AI_ADAPTER_PATH="$ADAPTER_ROOT/$role" \
    AI_API_KEY="$api_key" \
    "$PYTHON_BIN" "$ROOT/server.py" >"$LOG_ROOT/$role.log" 2>&1 &
  echo $!
}

INTERVIEWER_PID="$(run_role interviewer "${AI_INTERVIEWER_PORT:-8001}")"
EVALUATOR_PID="$(run_role evaluator "${AI_EVALUATOR_PORT:-8002}")"
trap 'kill "$INTERVIEWER_PID" "$EVALUATOR_PID" 2>/dev/null || true' EXIT INT TERM
wait "$INTERVIEWER_PID" "$EVALUATOR_PID"
