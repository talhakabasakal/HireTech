"""OpenAI-compatible ZeroGPU Space for one HireTech QLoRA adapter."""

from __future__ import annotations

import os
import time
import uuid
from typing import Any

import gradio as gr
import spaces
import torch
from fastapi import FastAPI, Header, HTTPException
from peft import PeftModel
from pydantic import BaseModel, Field
from transformers import AutoModelForCausalLM, AutoTokenizer, BitsAndBytesConfig


ROLE = os.getenv("AI_ROLE", "interviewer")
BASE_MODEL = os.getenv(
    "AI_MODEL_ID",
    "microsoft/Phi-4-mini-instruct" if ROLE == "interviewer" else "Qwen/Qwen3-4B",
)
ADAPTER_ID = os.getenv(
    "AI_ADAPTER_ID",
    "TalhaKa/hiretech-interviewer-qlora" if ROLE == "interviewer" else "TalhaKa/hiretech-evaluator-qlora",
)
API_KEY = os.getenv("AI_API_KEY", "")
MAX_NEW_TOKENS = int(os.getenv("AI_MAX_NEW_TOKENS", "2048"))
HF_TOKEN = os.getenv("HF_TOKEN") or os.getenv("HUGGINGFACE_HUB_TOKEN") or None
TOKENIZER: Any = None
MODEL: Any = None
DEVICE: torch.device | None = None


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


def _load_model() -> tuple[Any, Any, torch.device]:
    if not torch.cuda.is_available():
        raise RuntimeError("ZeroGPU did not provide a CUDA device")
    compute_dtype = torch.bfloat16 if torch.cuda.is_bf16_supported() else torch.float16
    quantization = BitsAndBytesConfig(
        load_in_4bit=True,
        bnb_4bit_quant_type="nf4",
        bnb_4bit_use_double_quant=True,
        bnb_4bit_compute_dtype=compute_dtype,
    )
    tokenizer = AutoTokenizer.from_pretrained(BASE_MODEL, use_fast=True, trust_remote_code=False, token=HF_TOKEN)
    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token
    model = AutoModelForCausalLM.from_pretrained(
        BASE_MODEL,
        quantization_config=quantization,
        device_map="auto",
        trust_remote_code=False,
        token=HF_TOKEN,
    )
    model = PeftModel.from_pretrained(model, ADAPTER_ID, is_trainable=False, token=HF_TOKEN)
    model.eval()
    return tokenizer, model, next(model.parameters()).device


@spaces.GPU(duration=180)
def _complete(messages: list[dict[str, str]], max_tokens: int, temperature: float | None) -> tuple[str, int, int]:
    global TOKENIZER, MODEL, DEVICE
    if TOKENIZER is None or MODEL is None or DEVICE is None:
        TOKENIZER, MODEL, DEVICE = _load_model()
    try:
        prompt = TOKENIZER.apply_chat_template(messages, tokenize=False, add_generation_prompt=True, enable_thinking=False)
    except TypeError:
        prompt = TOKENIZER.apply_chat_template(messages, tokenize=False, add_generation_prompt=True)
    encoded = TOKENIZER(prompt, return_tensors="pt")
    encoded = {key: value.to(DEVICE) for key, value in encoded.items()}
    limit = min(max_tokens, MAX_NEW_TOKENS)
    generation: dict[str, Any] = {
        **encoded,
        "max_new_tokens": limit,
        "do_sample": bool(temperature and temperature > 0),
        "pad_token_id": TOKENIZER.pad_token_id,
        "eos_token_id": TOKENIZER.eos_token_id,
    }
    if temperature and temperature > 0:
        generation["temperature"] = temperature
        generation["top_p"] = 0.9
    with torch.inference_mode():
        output = MODEL.generate(**generation)
    prompt_tokens = int(encoded["input_ids"].shape[-1])
    output_tokens = int(output.shape[-1] - prompt_tokens)
    content = _strip_thinking(TOKENIZER.decode(output[0][prompt_tokens:], skip_special_tokens=True))
    return content, prompt_tokens, output_tokens


api = FastAPI(title=f"HireTech {ROLE} AI")


@api.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok", "role": ROLE, "model": BASE_MODEL, "adapter": ADAPTER_ID}


@api.get("/v1/models")
def models() -> dict[str, Any]:
    return {"object": "list", "data": [{"id": BASE_MODEL, "object": "model", "owned_by": "hiretech"}]}


@api.post("/v1/chat/completions")
def chat_completions(request: ChatRequest, authorization: str | None = Header(default=None)) -> dict[str, Any]:
    if API_KEY and authorization != f"Bearer {API_KEY}":
        raise HTTPException(status_code=401, detail="invalid AI API key")
    if request.model and request.model != BASE_MODEL:
        raise HTTPException(status_code=400, detail="model is not served by this role endpoint")
    messages = [{"role": item.role, "content": _content(item.content)} for item in request.messages]
    started = time.monotonic()
    content, prompt_tokens, output_tokens = _complete(messages, request.max_tokens, request.temperature)
    return {
        "id": f"hiretech-{uuid.uuid4().hex}",
        "object": "chat.completion",
        "created": int(time.time()),
        "model": BASE_MODEL,
        "choices": [{"index": 0, "message": {"role": "assistant", "content": content}, "finish_reason": "stop"}],
        "usage": {"prompt_tokens": prompt_tokens, "completion_tokens": output_tokens, "total_tokens": prompt_tokens + output_tokens},
        "x_hiretech_latency_ms": round((time.monotonic() - started) * 1000, 2),
    }


demo = gr.Interface(
    fn=lambda prompt: _complete([{"role": "user", "content": prompt}], 512, 0)[0],
    inputs=gr.Textbox(label="Prompt"),
    outputs=gr.Textbox(label="Response"),
    title=f"HireTech {ROLE.title()} model",
)
app = gr.mount_gradio_app(api, demo, path="/")


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=7860)
