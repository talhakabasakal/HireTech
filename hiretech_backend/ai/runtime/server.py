"""Small OpenAI-compatible server for a HireTech QLoRA adapter.

Run one process per role. The Go backend talks to these processes through the
same /v1/chat/completions contract used by hosted providers.
"""

from __future__ import annotations

import os
import time
import uuid
from pathlib import Path
from typing import Any

import torch
from fastapi import FastAPI, Header, HTTPException
from pydantic import BaseModel, Field
from transformers import AutoModelForCausalLM, AutoTokenizer, BitsAndBytesConfig
from peft import PeftModel


ROLE = os.getenv("AI_ROLE", "interviewer")
BASE_MODEL = os.getenv(
    "AI_MODEL_ID",
    "microsoft/Phi-4-mini-instruct" if ROLE == "interviewer" else "Qwen/Qwen3-4B",
)
ADAPTER_PATH = Path(os.getenv("AI_ADAPTER_PATH", f"/app/adapters/{ROLE}"))
HOST = os.getenv("AI_HOST", "127.0.0.1")
PORT = int(os.getenv("AI_PORT", "8001"))
MAX_NEW_TOKENS = int(os.getenv("AI_MAX_NEW_TOKENS", "2048"))
API_KEY = os.getenv("AI_API_KEY", "")


class Message(BaseModel):
    role: str
    content: str | list[dict[str, Any]]


class ChatRequest(BaseModel):
    model: str = ""
    messages: list[Message] = Field(min_length=1)
    max_tokens: int = Field(default=1024, ge=1, le=32768)
    temperature: float | None = Field(default=None, ge=0, le=2)
    response_format: dict[str, Any] | None = None


def _content(value: str | list[dict[str, Any]]) -> str:
    if isinstance(value, str):
        return value
    return "\n".join(str(item.get("text", "")) for item in value if item.get("type") == "text")


def _strip_thinking(value: str) -> str:
    if "</think>" in value:
        value = value.split("</think>", 1)[1]
    return value.replace("<|endoftext|>", "").strip()


def _auth_ok(authorization: str | None) -> bool:
    return not API_KEY or authorization == f"Bearer {API_KEY}"


def _load_model() -> tuple[Any, Any, torch.device]:
    if not ADAPTER_PATH.is_dir():
        raise RuntimeError(f"QLoRA adapter directory not found: {ADAPTER_PATH}")
    if not torch.cuda.is_available():
        raise RuntimeError("CUDA GPU is required for the bundled QLoRA runtime")

    compute_dtype = torch.bfloat16 if torch.cuda.is_bf16_supported() else torch.float16
    quantization = BitsAndBytesConfig(
        load_in_4bit=True,
        bnb_4bit_quant_type="nf4",
        bnb_4bit_use_double_quant=True,
        bnb_4bit_compute_dtype=compute_dtype,
    )
    tokenizer = AutoTokenizer.from_pretrained(BASE_MODEL, use_fast=True, trust_remote_code=False)
    template_path = ADAPTER_PATH / "chat_template.jinja"
    if template_path.is_file():
        tokenizer.chat_template = template_path.read_text(encoding="utf-8")
    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token

    model = AutoModelForCausalLM.from_pretrained(
        BASE_MODEL,
        quantization_config=quantization,
        device_map="auto",
        trust_remote_code=False,
    )
    model = PeftModel.from_pretrained(model, str(ADAPTER_PATH), is_trainable=False)
    model.eval()
    return tokenizer, model, next(model.parameters()).device


app = FastAPI(title=f"HireTech {ROLE} AI", docs_url=None, redoc_url=None)
TOKENIZER, MODEL, DEVICE = _load_model()


@app.get("/health")
def health() -> dict[str, Any]:
    return {"status": "ok", "role": ROLE, "model": BASE_MODEL, "adapter": str(ADAPTER_PATH)}


@app.get("/v1/models")
def models() -> dict[str, Any]:
    return {"object": "list", "data": [{"id": BASE_MODEL, "object": "model", "owned_by": "hiretech"}]}


@app.post("/v1/chat/completions")
def chat_completions(request: ChatRequest, authorization: str | None = Header(default=None)) -> dict[str, Any]:
    if not _auth_ok(authorization):
        raise HTTPException(status_code=401, detail="invalid AI API key")
    if request.model and request.model != BASE_MODEL:
        raise HTTPException(status_code=400, detail="model is not served by this role endpoint")

    messages = [{"role": item.role, "content": _content(item.content)} for item in request.messages]
    try:
        try:
            prompt = TOKENIZER.apply_chat_template(
                messages, tokenize=False, add_generation_prompt=True, enable_thinking=False
            )
        except TypeError:
            prompt = TOKENIZER.apply_chat_template(messages, tokenize=False, add_generation_prompt=True)
    except Exception as exc:
        raise HTTPException(status_code=400, detail=f"invalid chat messages: {exc}") from exc

    encoded = TOKENIZER(prompt, return_tensors="pt")
    encoded = {key: value.to(DEVICE) for key, value in encoded.items()}
    limit = min(request.max_tokens, MAX_NEW_TOKENS)
    generation: dict[str, Any] = {
        **encoded,
        "max_new_tokens": limit,
        "do_sample": bool(request.temperature and request.temperature > 0),
        "pad_token_id": TOKENIZER.pad_token_id,
        "eos_token_id": TOKENIZER.eos_token_id,
    }
    if request.temperature and request.temperature > 0:
        generation["temperature"] = request.temperature
        generation["top_p"] = 0.9

    started = time.monotonic()
    with torch.inference_mode():
        output = MODEL.generate(**generation)
    prompt_tokens = int(encoded["input_ids"].shape[-1])
    output_tokens = int(output.shape[-1] - prompt_tokens)
    content = _strip_thinking(TOKENIZER.decode(output[0][prompt_tokens:], skip_special_tokens=True))
    return {
        "id": f"hiretech-{uuid.uuid4().hex}",
        "object": "chat.completion",
        "created": int(time.time()),
        "model": BASE_MODEL,
        "choices": [{"index": 0, "message": {"role": "assistant", "content": content}, "finish_reason": "stop"}],
        "usage": {"prompt_tokens": prompt_tokens, "completion_tokens": output_tokens, "total_tokens": prompt_tokens + output_tokens},
        "x_hiretech_latency_ms": round((time.monotonic() - started) * 1000, 2),
    }


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host=HOST, port=PORT)
