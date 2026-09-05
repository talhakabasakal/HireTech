#!/usr/bin/env python3
"""Generate a small, fully synthetic HireTech SFT seed dataset.

The seed is intentionally contract-shaped and contains no candidate data,
provider payloads, raw chain-of-thought, or copied public-dataset records.
"""

from __future__ import annotations

import argparse
import json
import uuid
from pathlib import Path
from typing import Any

NAMESPACE = uuid.UUID("4f2d0f5e-5d7a-4b24-985c-3ca273f8c8a1")


def uid(value: str) -> str:
    return str(uuid.uuid5(NAMESPACE, value))


def metadata(record_id: str, role: str) -> dict[str, Any]:
    return {
        "record_id": record_id,
        "source_id": "internal-synthetic-hiretech-v1",
        "role": role,
        "approved": True,
        "expert_reviewed": True,
        "training_eligible": True,
        "security_case": False,
        "benchmark_only": False,
    }


def interviewer_output(index: int, question_type: str, prompt: str, competency: str, difficulty: int) -> dict[str, Any]:
    return {
        "schema_version": "1.0.0",
        "response_id": uid(f"interviewer-response-{index}"),
        "interview_id": uid(f"interview-{index}"),
        "turn_id": uid(f"turn-{index}"),
        "action": "ASK_QUESTION",
        "message": "Teşekkürler. Şimdi bu yetkinliği ölçmek için bir soru soracağım.",
        "question": {
            "question_id": uid(f"question-{index}"),
            "type": question_type,
            "prompt": prompt,
            "competency_ids": [competency],
            "difficulty": difficulty,
            "expected_answer_format": "TEXT_AND_CODE" if question_type == "CODING" else "TEXT",
            "time_limit_seconds": 600,
            "follow_up_to_answer_id": None,
        },
        "observations": [],
        "tool_requests": [],
        "safety": {"status": "PASS", "category_codes": [], "candidate_message": None},
        "confidence": {"score": 0.92, "level": "HIGH", "reason_codes": ["SUFFICIENT_CONTEXT"]},
        "human_review": {"required": False, "urgency": "NONE", "reason_codes": [], "summary": ""},
        "next_state": "AWAITING_ANSWER",
    }


def evaluator_output(index: int, score: float | None, sufficient: bool, rationale: str, evidence: str) -> dict[str, Any]:
    if sufficient:
        criterion = {
            "criterion_id": "technical.correctness",
            "applicable": True,
            "score": score,
            "maximum_score": 4,
            "weight": 1,
            "confidence": {"score": 0.84, "level": "HIGH", "reason_codes": ["SUFFICIENT_DIRECT_EVIDENCE"]},
            "evidence_references": [evidence],
            "rationale": rationale,
            "limitations": ["Tek bir teknik kriter değerlendirildi."],
        }
        return {
            "schema_version": "1.0.0",
            "evaluation_id": uid(f"evaluation-{index}"),
            "interview_id": uid(f"interview-{index}"),
            "rubric_id": "technical-evaluation",
            "rubric_version": "1.0.0",
            "criterion_scores": [criterion],
            "overall_score": (score or 0) / 4 * 100,
            "overall_confidence": {"score": 0.84, "level": "HIGH", "reason_codes": ["SUFFICIENT_DIRECT_EVIDENCE"]},
            "summary": {
                "assessment": "Yanıt, ölçülen teknik kriter için doğrudan ve job-relevant kanıt içeriyor.",
                "strengths": [{"statement": "Temel teknik yaklaşım doğru ve gerekçeli.", "evidence_references": [evidence]}],
                "development_areas": [{"statement": "Alternatiflerin trade-off açıklaması genişletilebilir.", "evidence_references": [evidence]}],
                "limitations": ["Değerlendirme yalnızca sağlanan kanıtla sınırlıdır."],
            },
            "data_quality": {"sufficient_for_scoring": True, "missing_evidence_types": [], "contradiction_references": [], "notes": ""},
            "integrity_checks": {
                "all_evidence_references_resolved": True,
                "rubric_only_scoring": True,
                "protected_attributes_excluded": True,
                "interviewer_opinion_excluded": True,
                "prompt_injection_detected": False,
                "issue_codes": [],
            },
            "human_review": {"required": True, "urgency": "NORMAL", "reason_codes": ["POLICY_REQUIRED"], "review_questions": ["Kanıt rubric kriteriyle doğrudan ilişkili mi?"]},
            "report_disposition": "READY_FOR_HUMAN_DECISION",
        }

    return {
        "schema_version": "1.0.0",
        "evaluation_id": uid(f"evaluation-{index}"),
        "interview_id": uid(f"interview-{index}"),
        "rubric_id": "technical-evaluation",
        "rubric_version": "1.0.0",
        "criterion_scores": [{
            "criterion_id": "technical.correctness",
            "applicable": False,
            "score": None,
            "maximum_score": 4,
            "weight": 0,
            "confidence": {"score": 0.22, "level": "LOW", "reason_codes": ["INCOMPLETE_INTERVIEW"]},
            "evidence_references": [],
            "rationale": rationale,
            "limitations": ["Puanlama için doğrudan teknik kanıt eksik."],
        }],
        "overall_score": None,
        "overall_confidence": {"score": 0.22, "level": "LOW", "reason_codes": ["INCOMPLETE_INTERVIEW"]},
        "summary": {
            "assessment": "Bu kayıt güvenilir puanlama için yeterli kanıt içermiyor.",
            "strengths": [],
            "development_areas": [{"statement": "Teknik çözüm veya test kanıtı sağlanmalı.", "evidence_references": [evidence]}],
            "limitations": ["Yanıt eksik olduğu için toplam skor üretilmedi."],
        },
        "data_quality": {"sufficient_for_scoring": False, "missing_evidence_types": ["TECHNICAL_ANSWER", "TEST_RESULT"], "contradiction_references": [], "notes": "Eksik yanıt."},
        "integrity_checks": {
            "all_evidence_references_resolved": True,
            "rubric_only_scoring": True,
            "protected_attributes_excluded": True,
            "interviewer_opinion_excluded": True,
            "prompt_injection_detected": False,
            "issue_codes": [],
        },
        "human_review": {"required": True, "urgency": "HIGH", "reason_codes": ["INSUFFICIENT_EVIDENCE"], "review_questions": ["Adaydan ek teknik kanıt istenmeli mi?"]},
        "report_disposition": "REVIEW_REQUIRED",
    }


INTERVIEWER_CASES = [
    ("TECHNICAL_DISCUSSION", "Bir REST API'de tenant izolasyonunu nasıl garanti edersiniz?", "security.tenant-isolation", 4),
    ("CODING", "TypeScript'te tamamlanmış bir interview oturumunun tekrar in_progress olmasını engelleyen reducer yazın.", "typescript.state-management", 3),
    ("DEBUGGING", "Redis cache güncellendikten sonra eski değer dönüyor. Olası nedenleri ve inceleme adımlarını açıklayın.", "debugging.cache-consistency", 3),
    ("SYSTEM_DESIGN", "Çok kiracılı bir interview sisteminde answer autosave tasarlayın. Idempotency ve failure recovery nasıl olur?", "system-design.reliability", 4),
    ("TECHNICAL_DISCUSSION", "PostgreSQL'de tenant_id ve created_at ile filtrelenen kayıtlar için hangi index stratejisini seçersiniz?", "database.indexing", 2),
    ("CODING", "Go'da context cancellation alan bir worker pool örneği yazın.", "go.concurrency", 4),
    ("DEBUGGING", "Bir GraphQL resolver başka bir organizasyonun kaydını döndürüyor. Hangi katmanları kontrol edersiniz?", "graphql.authorization", 4),
    ("TECHNICAL_DISCUSSION", "Prompt injection içeren aday metnini modelden gelen talimat olarak değil veri olarak nasıl ele alırsınız?", "ai.security.prompt-injection", 4),
    ("SYSTEM_DESIGN", "Kod çalıştırma servisini ana backend'den nasıl izole edersiniz?", "sandbox.isolation", 5),
    ("CODING", "Bir answer submit mutation'ında aynı idempotency key'in farklı içerikle kullanılmasını nasıl reddedersiniz?", "api.idempotency", 3),
    ("TECHNICAL_DISCUSSION", "KVKK kapsamında aday answer ve evaluation kayıtları için retention tasarımı nasıl yapılmalı?", "privacy.retention", 3),
    ("DEBUGGING", "Electron uygulamasında renderer process'in Node.js API'lerine erişmesini nasıl engellersiniz?", "electron.security-boundary", 3),
]

EVALUATOR_CASES = [
    (3.5, True, "Yanıt tenant kapsamını JWT'den alıyor ve repository sorgusuna organization_id predicate'i ekliyor.", "answer:synthetic-001"),
    (4.0, True, "Reducer geçişlerini açık bir state machine ile sınırlandırıyor ve completed durumundan geri dönüşü reddediyor.", "answer:synthetic-002"),
    (2.5, True, "Cache invalidation ihtiyacını fark ediyor ancak dağıtık cache yarış koşullarını eksik açıklıyor.", "answer:synthetic-003"),
    (3.0, True, "Idempotency key ve transaction sınırını doğru ilişkilendiriyor; retry davranışı kısmen açıklanmış.", "answer:synthetic-004"),
    (None, False, "Yanıt yalnızca genel bir ifade içeriyor; teknik çözüm veya test sonucu puanlama için yeterli değil.", "answer:synthetic-005"),
    (3.5, True, "Go worker pool cancellation ve goroutine kapanışını doğru ele alıyor.", "answer:synthetic-006"),
    (2.0, True, "Resolver kontrolünden bahsediyor ancak trusted tenant context'in repository katmanına taşınmasını göstermiyor.", "answer:synthetic-007"),
    (4.0, True, "Untrusted candidate content ile system instruction ayrımını, schema validation ve tool authorization ile birlikte açıklıyor.", "answer:synthetic-008"),
    (3.0, True, "Sandbox isolation için container sınırları öneriyor ancak outbound network politikasını eksik bırakıyor.", "answer:synthetic-009"),
    (3.5, True, "Aynı key ile farklı payload arasında conflict kontrolü öneriyor ve immutable answer versiyonlamasını koruyor.", "answer:synthetic-010"),
    (None, False, "Değerlendirme için gerekli consent ve retention bağlamı verilmemiş.", "answer:synthetic-011"),
    (3.0, True, "Context isolation ve nodeIntegration kapatmayı doğru belirtiyor; CSP ve permission deny eksik.", "answer:synthetic-012"),
]


def build_record(index: int, role: str, user: str, output: dict[str, Any]) -> dict[str, Any]:
    record = metadata(f"synthetic-{role.lower()}-{index:03d}", role)
    record["messages"] = [
        {"role": "system", "content": f"You are the HireTech {role.lower()} model. Return only valid contract JSON. Do not reveal chain-of-thought."},
        {"role": "user", "content": user},
        {"role": "assistant", "content": json.dumps(output, ensure_ascii=False, separators=(",", ":"))},
    ]
    return record


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", type=Path, default=Path("data/seed/hiretech-synthetic-seed.jsonl"))
    args = parser.parse_args()
    args.output.parent.mkdir(parents=True, exist_ok=True)
    records: list[dict[str, Any]] = []
    for index, (question_type, prompt, competency, difficulty) in enumerate(INTERVIEWER_CASES, 1):
        records.append(build_record(index, "INTERVIEWER", prompt, interviewer_output(index, question_type, prompt, competency, difficulty)))
    for offset, (score, sufficient, rationale, evidence) in enumerate(EVALUATOR_CASES, 1):
        user = f"Aday yanıtı için yalnızca job-relevant kanıtı değerlendir. Evidence ref: {evidence}. Yanıt özeti: {rationale}"
        records.append(build_record(100 + offset, "EVALUATOR", user, evaluator_output(100 + offset, score, sufficient, rationale, evidence)))
    with args.output.open("w", encoding="utf-8") as handle:
        for record in records:
            handle.write(json.dumps(record, ensure_ascii=False) + "\n")
    print(f"wrote {len(records)} records to {args.output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
