"""Fail-closed output guard for the evaluator Space."""
from __future__ import annotations
import json

REQUIRED = {"schema_version", "evaluation_id", "interview_id", "rubric_id", "rubric_version", "criterion_scores", "overall_score", "overall_confidence", "summary", "data_quality", "integrity_checks", "human_review", "report_disposition"}
INSTRUCTION = "Return exactly one JSON object for the HireTech evaluator-output contract v1.0.0; use every required field, no markdown or extra fields, only resolved job-relevant evidence, and never make a hiring decision."

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
        raise ValueError("evaluator contract mismatch")
    if value.get("report_disposition") == "READY_FOR_HUMAN_DECISION" and value.get("human_review", {}).get("required") is not True:
        raise ValueError("human decision report requires human review")
