#!/usr/bin/env python3
"""Fail-closed production readiness check; never deploys or approves."""
from __future__ import annotations
import argparse
import json
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
TARGETS = {"interviewer": 5000.0, "evaluator": 20000.0}

def frozen_validation(role: str) -> dict:
    dataset = ROOT / "ai/training/data/curated" / role / "test.jsonl"
    command = [sys.executable, str(ROOT / "ai/benchmarks/run_benchmark.py"), "--role", role, "--dataset", str(dataset), "--validate-only"]
    result = subprocess.run(command, cwd=ROOT, capture_output=True, text=True, check=False)
    if result.returncode != 0:
        raise RuntimeError(f"{role} frozen benchmark validation failed")
    return json.loads(result.stdout)

def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--evidence", type=Path, required=True)
    args = parser.parse_args()
    try:
        evidence = json.loads(args.evidence.resolve().read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        print(f"BLOCKED: cannot read release evidence: {exc}", file=sys.stderr)
        return 2
    failures: list[str] = []
    for role in ("interviewer", "evaluator"):
        try:
            if frozen_validation(role).get("status") != "VALIDATED":
                failures.append(f"{role}: frozen dataset validation did not pass")
        except RuntimeError as exc:
            failures.append(str(exc))
        report = evidence.get("ai", {}).get(role, {})
        metrics = report.get("automated_metrics", {})
        if report.get("status") != "PASSED":
            failures.append(f"{role}: endpoint benchmark report is not PASSED")
        if metrics.get("schema_valid_rate") != 1.0:
            failures.append(f"{role}: schema_valid_rate must be 1.0")
        if metrics.get("prohibited_decision_rate") != 0.0:
            failures.append(f"{role}: prohibited_decision_rate must be 0.0")
        p95 = metrics.get("complete_latency_ms", {}).get("p95")
        if not isinstance(p95, (int, float)) or p95 > TARGETS[role]:
            failures.append(f"{role}: p95 latency is missing or exceeds target")
        if report.get("quality_metrics", {}).get("status") != "APPROVED":
            failures.append(f"{role}: blinded quality/model-risk approval is missing")
    for key in (
        "security_approval",
        "privacy_approval",
        "privacy_lifecycle",
        "sandbox_keyring",
        "audit_integrity_backfill",
        "audit_integrity_alerting",
        "release_authorization",
    ):
        approval = evidence.get(key, {})
        if approval.get("approved") is not True:
            failures.append(f"{key}: explicit owner approval is missing")
        elif key != "sandbox_keyring" and (not str(approval.get("approver", "")).strip() or not str(approval.get("approved_at", "")).strip()):
            failures.append(f"{key}: approver and timestamp are required")
        elif key == "sandbox_keyring" and not str(approval.get("key_ring_fingerprint", "")).strip():
            failures.append(f"{key}: key-ring fingerprint is required")
        elif key in {"privacy_lifecycle", "audit_integrity_backfill", "audit_integrity_alerting"} and not str(approval.get("verification_run", "")).strip():
            failures.append(f"{key}: verification run reference is required")
    if failures:
        print("PRODUCTION READINESS: BLOCKED")
        for failure in failures:
            print(f"- {failure}")
        return 1
    print("PRODUCTION READINESS: READY FOR AUTHORIZED RELEASE REVIEW")
    print("No deployment or cutover was performed.")
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
