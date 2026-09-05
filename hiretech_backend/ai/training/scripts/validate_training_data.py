#!/usr/bin/env python3
"""Validate curated JSONL against HireTech output contracts."""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any

EMAIL_RE = re.compile(r"\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b", re.I)
ROOT = Path(__file__).resolve().parents[2]
CONTRACTS = ROOT / "contracts"


def load_validator(schema_path: Path):
    try:
        from jsonschema import Draft202012Validator, FormatChecker
    except ImportError:
        print("jsonschema is required: pip install -r requirements.txt", file=sys.stderr)
        raise SystemExit(2)
    schema = json.loads(schema_path.read_text(encoding="utf-8"))
    return Draft202012Validator(schema, format_checker=FormatChecker())


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--role", choices=("interviewer", "evaluator"), required=True)
    parser.add_argument("--data-dir", type=Path, default=None)
    parser.add_argument("--contracts-dir", type=Path, default=CONTRACTS)
    args = parser.parse_args()

    data_dir = args.data_dir or (ROOT / "training" / "data" / "curated" / args.role)
    schema_name = f"{args.role}-output.schema.json"
    validator = load_validator(args.contracts_dir / schema_name)
    errors: list[str] = []
    total = 0
    if not data_dir.is_dir():
        print(f"Veri klasörü bulunamadı: {data_dir}", file=sys.stderr)
        return 2

    for path in sorted(data_dir.glob("*.jsonl")):
        with path.open(encoding="utf-8") as handle:
            for line_no, line in enumerate(handle, 1):
                if not line.strip():
                    continue
                total += 1
                try:
                    record: dict[str, Any] = json.loads(line)
                    messages = record["messages"]
                    if messages[-1]["role"] != "assistant":
                        raise ValueError("last message is not assistant")
                    content = messages[-1]["content"]
                    if EMAIL_RE.search(content):
                        raise ValueError("possible email/PII found")
                    output = json.loads(content)
                    found = list(validator.iter_errors(output))
                    if found:
                        first = found[0]
                        location = ".".join(str(part) for part in first.absolute_path)
                        raise ValueError(f"{location}: {first.message}")
                except (KeyError, IndexError, TypeError, ValueError, json.JSONDecodeError) as exc:
                    errors.append(f"{path}:{line_no}: {exc}")

    if total == 0:
        errors.append("no records found")
    print(f"role={args.role} records={total} errors={len(errors)}")
    for error in errors[:20]:
        print(error, file=sys.stderr)
    return 1 if errors else 0


if __name__ == "__main__":
    raise SystemExit(main())
