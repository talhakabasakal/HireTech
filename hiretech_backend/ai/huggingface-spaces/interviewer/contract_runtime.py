"""Fail-closed output guard for the interviewer Space."""
from __future__ import annotations
import json

REQUIRED = {"schema_version", "response_id", "interview_id", "turn_id", "action", "message", "question", "observations", "tool_requests", "safety", "confidence", "human_review", "next_state"}
INSTRUCTION = "Return exactly one JSON object for the HireTech interviewer-output contract v1.0.0; use every required field, no markdown or extra fields, no chain-of-thought, and never make a hiring decision."

def add_contract_instruction(messages: list[dict[str, str]], role: str) -> list[dict[str, str]]:
    if any(item.get("role") == "system" and INSTRUCTION in item.get("content", "") for item in messages):
        return messages
    return [{"role": "system", "content": INSTRUCTION}, *messages]

def validate_contract_text(content: str, role: str) -> None:
    try:
        value = json.loads(content.strip())
    except json.JSONDecodeError as exc:
        raise ValueError("invalid JSON") from exc
    if not isinstance(value, dict) or set(value) != REQUIRED or value.get("schema_version") != "1.0.0":
        raise ValueError("interviewer contract mismatch")
