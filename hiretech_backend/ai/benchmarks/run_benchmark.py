#!/usr/bin/env python3
"""Run a bounded, contract-first benchmark against an OpenAI-compatible AI endpoint.

The runner intentionally measures only evidence it can establish automatically:
JSON/schema validity, prohibited-decision output, latency, and token usage. Expert
quality, bias, evidence faithfulness, and security review remain explicit gates and
are never synthesized from model output.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import statistics
import sys
import time
from pathlib import Path
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.parse import urlparse
from urllib.request import Request, urlopen


ROOT = Path(__file__).resolve().parents[2]
AI_ROOT = ROOT / "ai"
DEFAULT_MANIFEST = AI_ROOT / "training/data/curated/manifest.json"
DEFAULT_DATASET_ROOT = AI_ROOT / "training/data/curated"
SCHEMA_BY_ROLE = {
    "interviewer": AI_ROOT / "contracts/interviewer-output.schema.json",
    "evaluator": AI_ROOT / "contracts/evaluator-output.schema.json",
}
ROLE_TARGETS = {"interviewer": 5.0, "evaluator": 20.0}
PROHIBITED_DECISION_PATTERNS = (
    r"\bno[- ]?hire\b",
    r"\bhire\b",
    r"\baccept\b",
    r"\breject\b",
    r"işe\s+al",
    r"işe\s+alma",
    r"reddet",
    r"reddedil",
)


class BenchmarkError(Exception):
    """Expected, user-actionable benchmark failure."""


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--role", choices=sorted(SCHEMA_BY_ROLE), required=True)
    parser.add_argument("--dataset", type=Path, required=True, help="Frozen JSONL dataset file")
    parser.add_argument("--endpoint", help="AI base URL or /v1/chat/completions URL")
    parser.add_argument("--model", help="Exact model identifier served by the endpoint")
    parser.add_argument("--manifest", type=Path, default=DEFAULT_MANIFEST)
    parser.add_argument("--expected-sha256", help="Required for datasets not listed in the manifest")
    parser.add_argument("--output", type=Path, help="Write a report JSON file")
    parser.add_argument("--validate-only", action="store_true", help="Validate frozen inputs without calling an endpoint")
    parser.add_argument("--api-key-env", default="AI_BENCHMARK_API_KEY", help="Environment variable containing the bearer token")
    parser.add_argument("--timeout-seconds", type=float, default=60.0)
    parser.add_argument("--max-attempts", type=int, default=1, help="Attempts per record, maximum 3")
    parser.add_argument("--max-tokens", type=int, default=2048)
    parser.add_argument("--max-records", type=int, default=1000)
    parser.add_argument("--registry-version", default="UNSPECIFIED")
    parser.add_argument("--prompt-version", default="UNSPECIFIED")
    parser.add_argument("--rubric-version", default="UNSPECIFIED")
    parser.add_argument("--region", default="UNSPECIFIED")
    return parser.parse_args()


def load_json(path: Path) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise BenchmarkError(f"cannot read JSON file: {path}: {exc}") from exc
    if not isinstance(value, dict):
        raise BenchmarkError(f"JSON root must be an object: {path}")
    return value


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    try:
        with path.open("rb") as handle:
            for chunk in iter(lambda: handle.read(1024 * 1024), b""):
                digest.update(chunk)
    except OSError as exc:
        raise BenchmarkError(f"cannot hash dataset: {path}: {exc}") from exc
    return digest.hexdigest()


def expected_hash(dataset: Path, manifest_path: Path, role: str, supplied: str | None) -> str:
    if supplied:
        if not re.fullmatch(r"[0-9a-f]{64}", supplied):
            raise BenchmarkError("--expected-sha256 must be a lowercase SHA-256 digest")
        return supplied
    manifest = load_json(manifest_path)
    try:
        relative = dataset.resolve().relative_to(DEFAULT_DATASET_ROOT.resolve())
    except ValueError as exc:
        raise BenchmarkError("dataset is outside the curated set; provide --expected-sha256") from exc
    parts = relative.parts
    if len(parts) != 2 or parts[0] != role or not parts[1].endswith(".jsonl"):
        raise BenchmarkError("curated dataset path must be <role>/<split>.jsonl")
    split = parts[1][:-6]
    try:
        expected = manifest["splits"][role][split]["sha256"]
    except (KeyError, TypeError) as exc:
        raise BenchmarkError(f"manifest has no frozen hash for {role}/{split}") from exc
    if not isinstance(expected, str):
        raise BenchmarkError("manifest SHA-256 value is invalid")
    return expected


def load_records(dataset: Path, role: str, max_records: int) -> list[dict[str, Any]]:
    if max_records < 1 or max_records > 10_000:
        raise BenchmarkError("--max-records must be between 1 and 10000")
    if not dataset.is_file():
        raise BenchmarkError(f"dataset does not exist: {dataset}")
    records: list[dict[str, Any]] = []
    try:
        lines = dataset.read_text(encoding="utf-8").splitlines()
    except OSError as exc:
        raise BenchmarkError(f"cannot read dataset: {dataset}: {exc}") from exc
    for line_number, line in enumerate(lines, 1):
        if not line.strip():
            continue
        if len(records) >= max_records:
            raise BenchmarkError(f"dataset exceeds --max-records={max_records}")
        try:
            record = json.loads(line)
        except json.JSONDecodeError as exc:
            raise BenchmarkError(f"invalid JSONL at line {line_number}: {exc}") from exc
        if not isinstance(record, dict) or not isinstance(record.get("record_id"), str):
            raise BenchmarkError(f"record_id is required at line {line_number}")
        messages = record.get("messages")
        if not isinstance(messages, list) or not messages:
            raise BenchmarkError(f"messages are required for {record['record_id']}")
        if not isinstance(messages[-1], dict) or messages[-1].get("role") != "assistant":
            raise BenchmarkError(f"last message must be the frozen reference assistant output for {record['record_id']}")
        prompt = messages[:-1]
        if not prompt or any(not isinstance(item, dict) or item.get("role") not in {"system", "user", "assistant"} for item in prompt):
            raise BenchmarkError(f"invalid prompt messages for {record['record_id']}")
        if role == "interviewer" and "interviewer" not in record["record_id"]:
            raise BenchmarkError(f"record role mismatch for {record['record_id']}")
        if role == "evaluator" and "evaluator" not in record["record_id"]:
            raise BenchmarkError(f"record role mismatch for {record['record_id']}")
        reference = messages[-1].get("content")
        if not isinstance(reference, str):
            raise BenchmarkError(f"frozen assistant reference must be text for {record['record_id']}")
        try:
            reference_output = json.loads(reference)
        except json.JSONDecodeError as exc:
            raise BenchmarkError(f"frozen assistant reference is invalid JSON for {record['record_id']}: line {exc.lineno}") from exc
        records.append({"record_id": record["record_id"], "messages": prompt, "reference_output": reference_output})
    if not records:
        raise BenchmarkError("dataset contains no records")
    return records


def validator_for(schema_path: Path) -> Any:
    try:
        from jsonschema import Draft202012Validator, FormatChecker
    except ImportError as exc:
        raise BenchmarkError("jsonschema is required; install ai/benchmarks/requirements.txt") from exc
    schema = load_json(schema_path)
    validator = Draft202012Validator(schema, format_checker=FormatChecker())
    try:
        validator.check_schema(schema)
    except Exception as exc:
        raise BenchmarkError(f"invalid output schema: {schema_path}: {exc}") from exc
    return validator


def validate_inputs(args: argparse.Namespace) -> tuple[list[dict[str, Any]], str, Any]:
    dataset = args.dataset.resolve()
    expected = expected_hash(dataset, args.manifest.resolve(), args.role, args.expected_sha256)
    actual = sha256_file(dataset)
    if actual != expected:
        raise BenchmarkError(f"frozen dataset hash mismatch: expected {expected}, got {actual}")
    records = load_records(dataset, args.role, args.max_records)
    validator = validator_for(SCHEMA_BY_ROLE[args.role])
    for record in records:
        errors = sorted(validator.iter_errors(record["reference_output"]), key=lambda error: str(list(error.path)))
        if errors:
            location = ".".join(str(item) for item in errors[0].path) or "$"
            raise BenchmarkError(f"frozen assistant reference fails {args.role} schema for {record['record_id']} at {location}")
    return records, actual, validator


def completion_url(endpoint: str) -> str:
    parsed = urlparse(endpoint)
    if parsed.scheme not in {"http", "https"} or parsed.username or parsed.password or parsed.query or parsed.fragment:
        raise BenchmarkError("--endpoint must be an http(s) URL without credentials, query, or fragment")
    base = endpoint.rstrip("/")
    if base.endswith("/chat/completions"):
        return base
    return f"{base}/chat/completions" if base.endswith("/v1") else f"{base}/v1/chat/completions"


def response_content(payload: dict[str, Any]) -> str:
    try:
        content = payload["choices"][0]["message"]["content"]
    except (KeyError, IndexError, TypeError) as exc:
        raise BenchmarkError("endpoint response has no assistant content") from exc
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        return "".join(str(part.get("text", "")) for part in content if isinstance(part, dict) and part.get("type") == "text")
    raise BenchmarkError("endpoint assistant content is not text")


def parse_json_content(content: str) -> tuple[Any | None, str | None]:
    cleaned = content.strip()
    if cleaned.startswith("```"):
        cleaned = re.sub(r"^```(?:json)?\s*|\s*```$", "", cleaned, flags=re.IGNORECASE | re.DOTALL).strip()
    try:
        return json.loads(cleaned), None
    except json.JSONDecodeError as exc:
        return None, f"invalid JSON: line {exc.lineno} column {exc.colno}"


def call_endpoint(url: str, model: str, messages: list[dict[str, Any]], api_key: str, timeout: float, max_tokens: int) -> tuple[dict[str, Any], float]:
    body = json.dumps(
        {
            "model": model,
            "messages": messages,
            "max_tokens": max_tokens,
            "temperature": 0,
            "response_format": {"type": "json_object"},
        },
        ensure_ascii=False,
    ).encode("utf-8")
    headers = {"Content-Type": "application/json", "Accept": "application/json"}
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"
    request = Request(url, data=body, headers=headers, method="POST")
    started = time.perf_counter()
    try:
        with urlopen(request, timeout=timeout) as response:
            raw = response.read()
    except (HTTPError, URLError, TimeoutError, OSError) as exc:
        raise BenchmarkError(f"endpoint request failed: {type(exc).__name__}") from exc
    elapsed_ms = (time.perf_counter() - started) * 1000
    try:
        payload = json.loads(raw.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise BenchmarkError("endpoint returned invalid JSON") from exc
    if not isinstance(payload, dict):
        raise BenchmarkError("endpoint response root is not an object")
    return payload, elapsed_ms


def percentile(values: list[float], fraction: float) -> float | None:
    if not values:
        return None
    ordered = sorted(values)
    index = min(len(ordered) - 1, max(0, int((len(ordered) - 1) * fraction)))
    return round(ordered[index], 2)


def run(args: argparse.Namespace) -> dict[str, Any]:
    records, dataset_hash, validator = validate_inputs(args)
    report: dict[str, Any] = {
        "status": "VALIDATED" if args.validate_only else "FAILED",
        "role": args.role,
        "dataset": str(args.dataset.resolve()),
        "dataset_sha256": dataset_hash,
        "record_count": len(records),
        "model": args.model or "UNSPECIFIED",
        "endpoint": completion_url(args.endpoint) if args.endpoint else "UNSPECIFIED",
        "metadata": {
            "registry_version": args.registry_version,
            "prompt_version": args.prompt_version,
            "rubric_version": args.rubric_version,
            "region": args.region,
        },
        "automated_metrics": {
            "raw_json_rate": None,
            "schema_valid_rate": None,
            "prohibited_decision_rate": None,
            "complete_latency_ms": {"p50": None, "p95": None},
            "server_latency_ms": {"p50": None, "p95": None},
            "prompt_tokens": 0,
            "completion_tokens": 0,
            "total_tokens": 0,
        },
        "quality_metrics": {"status": "NOT_COMPUTED", "reason": "Requires blinded expert review and approved labels."},
        "records": [],
    }
    if args.validate_only:
        return report
    if not args.endpoint or not args.model:
        raise BenchmarkError("--endpoint and --model are required unless --validate-only is used")
    if args.timeout_seconds <= 0 or args.timeout_seconds > 300:
        raise BenchmarkError("--timeout-seconds must be between 0 and 300")
    if args.max_attempts < 1 or args.max_attempts > 3:
        raise BenchmarkError("--max-attempts must be between 1 and 3")
    if args.max_tokens < 1 or args.max_tokens > 32768:
        raise BenchmarkError("--max-tokens must be between 1 and 32768")

    url = completion_url(args.endpoint)
    api_key = os.environ.get(args.api_key_env, "")
    for record in records:
        result: dict[str, Any] = {"record_id": record["record_id"], "attempts": 0, "status": "FAILED"}
        for attempt in range(1, args.max_attempts + 1):
            result["attempts"] = attempt
            try:
                payload, elapsed_ms = call_endpoint(url, args.model, record["messages"], api_key, args.timeout_seconds, args.max_tokens)
                content = response_content(payload)
                parsed, parse_error = parse_json_content(content)
                raw_json = parsed is not None
                schema_errors = [] if parsed is None else sorted(validator.iter_errors(parsed), key=lambda error: str(list(error.path)))
                schema_valid = raw_json and not schema_errors
                serialized = json.dumps(parsed, ensure_ascii=False).lower() if parsed is not None else ""
                prohibited = any(re.search(pattern, serialized, flags=re.IGNORECASE) for pattern in PROHIBITED_DECISION_PATTERNS)
                usage = payload.get("usage") if isinstance(payload.get("usage"), dict) else {}
                prompt_tokens = int(usage.get("prompt_tokens", 0) or 0)
                completion_tokens = int(usage.get("completion_tokens", 0) or 0)
                total_tokens = int(usage.get("total_tokens", prompt_tokens + completion_tokens) or 0)
                server_ms = payload.get("x_hiretech_latency_ms")
                result.update(
                    {
                        "status": "PASS" if schema_valid and not prohibited else "FAILED",
                        "raw_json": raw_json,
                        "schema_valid": schema_valid,
                        "prohibited_decision": prohibited,
                        "complete_latency_ms": round(elapsed_ms, 2),
                        "server_latency_ms": server_ms if isinstance(server_ms, (int, float)) else None,
                        "prompt_tokens": prompt_tokens,
                        "completion_tokens": completion_tokens,
                        "total_tokens": total_tokens,
                        "error": parse_error if parse_error else ("schema validation failed" if not schema_valid else None),
                    }
                )
                report["automated_metrics"]["prompt_tokens"] += prompt_tokens
                report["automated_metrics"]["completion_tokens"] += completion_tokens
                report["automated_metrics"]["total_tokens"] += total_tokens
                break
            except BenchmarkError as exc:
                result["error"] = str(exc)
                if attempt == args.max_attempts:
                    break
        report["records"].append(result)

    count = len(records)
    raw_json_count = sum(1 for item in report["records"] if item.get("raw_json") is True)
    schema_valid_count = sum(1 for item in report["records"] if item.get("schema_valid") is True)
    prohibited_count = sum(1 for item in report["records"] if item.get("prohibited_decision") is True)
    complete_latencies = [float(item["complete_latency_ms"]) for item in report["records"] if isinstance(item.get("complete_latency_ms"), (int, float))]
    server_latencies = [float(item["server_latency_ms"]) for item in report["records"] if isinstance(item.get("server_latency_ms"), (int, float))]
    report["automated_metrics"].update(
        {
            "raw_json_rate": round(raw_json_count / count, 4),
            "schema_valid_rate": round(schema_valid_count / count, 4),
            "prohibited_decision_rate": round(prohibited_count / count, 4),
            "complete_latency_ms": {"p50": percentile(complete_latencies, 0.50), "p95": percentile(complete_latencies, 0.95)},
            "server_latency_ms": {"p50": percentile(server_latencies, 0.50), "p95": percentile(server_latencies, 0.95)},
        }
    )
    report["status"] = "PASSED" if all(item["status"] == "PASS" for item in report["records"]) else "FAILED"
    report["release_gate"] = {
        "status": "BLOCKED",
        "reason": "Automated contract/latency evidence is not a release approval; expert, security, privacy, and model-risk gates remain required.",
        "latency_target_p95_seconds": ROLE_TARGETS[args.role],
    }
    return report


def main() -> int:
    args = parse_args()
    try:
        report = run(args)
        serialized = json.dumps(report, ensure_ascii=False, indent=2) + "\n"
        if args.output:
            args.output.parent.mkdir(parents=True, exist_ok=True)
            args.output.write_text(serialized, encoding="utf-8")
        print(serialized, end="")
        return 0 if report["status"] in {"VALIDATED", "PASSED"} else 1
    except BenchmarkError as exc:
        print(f"benchmark blocked: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
