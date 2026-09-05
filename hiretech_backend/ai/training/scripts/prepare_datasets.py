#!/usr/bin/env python3
"""Turn reviewed canonical JSONL records into deterministic role datasets."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
from collections import Counter
from pathlib import Path
from typing import Any

EMAIL_RE = re.compile(r"\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b", re.I)
ROLES = {"INTERVIEWER", "EVALUATOR"}


def read_records(root: Path) -> list[dict[str, Any]]:
    records: list[dict[str, Any]] = []
    for path in sorted(root.rglob("*")):
        if path.suffix not in {".json", ".jsonl"} or not path.is_file():
            continue
        with path.open(encoding="utf-8") as handle:
            if path.suffix == ".jsonl":
                for line_no, line in enumerate(handle, 1):
                    if line.strip():
                        value = json.loads(line)
                        if not isinstance(value, dict):
                            raise ValueError(f"{path}:{line_no}: expected object")
                        records.append(value)
            else:
                value = json.load(handle)
                if isinstance(value, dict) and isinstance(value.get("records"), list):
                    value = value["records"]
                if not isinstance(value, list):
                    raise ValueError(f"{path}: expected a JSON array or records array")
                records.extend(value)
    return records


def messages_of(record: dict[str, Any]) -> list[dict[str, str]]:
    messages = record.get("messages")
    if not isinstance(messages, list):
        system = record.get("system")
        user = record.get("user")
        assistant = record.get("assistant")
        messages = [
            {"role": "system", "content": system},
            {"role": "user", "content": user},
            {"role": "assistant", "content": assistant},
        ]
    return messages


def canonical_messages(record: dict[str, Any]) -> list[dict[str, str]]:
    messages = messages_of(record)
    if not messages or messages[-1].get("role") != "assistant":
        raise ValueError("last message must be assistant")
    result = []
    for message in messages:
        role = message.get("role")
        content = message.get("content")
        if role not in {"system", "user", "assistant"} or not isinstance(content, str) or not content.strip():
            raise ValueError("messages must contain non-empty system/user/assistant strings")
        result.append({"role": role, "content": content.strip()})
    return result


def digest(messages: list[dict[str, str]]) -> str:
    payload = json.dumps(messages, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(payload.encode("utf-8")).hexdigest()


def split_for(record: dict[str, Any], content_hash: str) -> str:
    group = str(record.get("split_group") or f"{record.get('source_id', 'unknown')}:{record.get('record_id', content_hash)}")
    bucket = int(hashlib.sha256(group.encode("utf-8")).hexdigest()[:8], 16) % 100
    if bucket < 80:
        return "train"
    if bucket < 90:
        return "validation"
    return "test"


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--manifest", type=Path, required=True)
    args = parser.parse_args()

    seen: set[str] = set()
    buckets: dict[str, dict[str, list[dict[str, Any]]]] = {
        role: {split: [] for split in ("train", "validation", "test")} for role in ROLES
    }
    skipped = Counter()

    for record in read_records(args.input):
        role = record.get("role")
        if role not in ROLES:
            skipped["unsupported_role"] += 1
            continue
        if not all(record.get(flag) is True for flag in ("approved", "expert_reviewed", "training_eligible")):
            skipped["not_approved_or_reviewed"] += 1
            continue
        if record.get("security_case") is True or record.get("benchmark_only") is True:
            skipped["security_or_benchmark"] += 1
            continue
        try:
            messages = canonical_messages(record)
        except (TypeError, ValueError):
            skipped["invalid_messages"] += 1
            continue
        joined = "\n".join(message["content"] for message in messages)
        if EMAIL_RE.search(joined):
            skipped["possible_pii_email"] += 1
            continue
        content_hash = digest(messages)
        if content_hash in seen:
            skipped["duplicate"] += 1
            continue
        seen.add(content_hash)
        split = split_for(record, content_hash)
        buckets[role][split].append({
            "record_id": str(record.get("record_id") or content_hash[:16]),
            "source_id": str(record.get("source_id") or "unknown"),
            "messages": messages,
        })

    args.output.mkdir(parents=True, exist_ok=True)
    manifest: dict[str, Any] = {"format_version": "1.0.0", "splits": {}, "skipped": dict(skipped)}
    for role, splits in buckets.items():
        role_dir = args.output / role.lower()
        role_dir.mkdir(parents=True, exist_ok=True)
        manifest["splits"][role.lower()] = {}
        for split, values in splits.items():
            target = role_dir / f"{split}.jsonl"
            with target.open("w", encoding="utf-8") as handle:
                for value in values:
                    handle.write(json.dumps(value, ensure_ascii=False) + "\n")
            manifest["splits"][role.lower()][split] = {
                "records": len(values),
                "sha256": hashlib.sha256(target.read_bytes()).hexdigest(),
            }
    args.manifest.parent.mkdir(parents=True, exist_ok=True)
    args.manifest.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(manifest, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
