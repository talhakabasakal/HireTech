#!/usr/bin/env python3
"""Train one role-specific QLoRA adapter for HireTech."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

MODELS = {
    "interviewer": "microsoft/Phi-4-mini-instruct",
    "evaluator": "Qwen/Qwen3-4B",
}


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--role", choices=MODELS, required=True)
    parser.add_argument("--data-dir", type=Path, required=True)
    parser.add_argument("--output-dir", type=Path, required=True)
    parser.add_argument("--model-id", default=None)
    parser.add_argument("--trust-remote-code", action="store_true")
    parser.add_argument("--max-length", type=int, default=None)
    parser.add_argument("--epochs", type=float, default=2.0)
    parser.add_argument("--gradient-accumulation", type=int, default=16)
    parser.add_argument("--resume-from-checkpoint", default=None)
    args = parser.parse_args()

    try:
        import torch
        from datasets import load_dataset
        from peft import LoraConfig, prepare_model_for_kbit_training
        from transformers import AutoModelForCausalLM, AutoTokenizer, BitsAndBytesConfig
        from trl import SFTConfig, SFTTrainer
    except ImportError as exc:
        print(f"Eksik paket: {exc}. Önce pip install -r requirements.txt çalıştırın.", file=sys.stderr)
        return 2

    if not torch.cuda.is_available():
        print("CUDA destekli GPU bulunamadı; QLoRA eğitimi başlatılmadı.", file=sys.stderr)
        return 2

    model_id = args.model_id or MODELS[args.role]
    max_length = args.max_length or (2048 if args.role == "interviewer" else 4096)
    args.output_dir.mkdir(parents=True, exist_ok=True)

    data_files = {
        "train": str(args.data_dir / "train.jsonl"),
        "validation": str(args.data_dir / "validation.jsonl"),
    }
    dataset = load_dataset("json", data_files=data_files)
    tokenizer = AutoTokenizer.from_pretrained(
        model_id, trust_remote_code=(args.trust_remote_code or model_id == "microsoft/Phi-4-mini-instruct"), use_fast=True
    )
    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token

    quantization = BitsAndBytesConfig(
        load_in_4bit=True,
        bnb_4bit_quant_type="nf4",
        bnb_4bit_use_double_quant=True,
        bnb_4bit_compute_dtype=torch.bfloat16 if torch.cuda.is_bf16_supported() else torch.float16,
    )
    model = AutoModelForCausalLM.from_pretrained(
        model_id,
        quantization_config=quantization,
        device_map="auto",
        trust_remote_code=(args.trust_remote_code or model_id == "microsoft/Phi-4-mini-instruct"),
    )
    model = prepare_model_for_kbit_training(model)
    model.config.use_cache = False

    assistant_only_loss = "{% generation %}" in (tokenizer.chat_template or "")
    if not assistant_only_loss:
        print("Uyarı: tokenizer chat template generation marker içermiyor; full-sequence loss kullanılacak.", file=sys.stderr)

    lora = LoraConfig(
        r=16,
        lora_alpha=32,
        lora_dropout=0.05,
        bias="none",
        task_type="CAUSAL_LM",
        target_modules="all-linear",
    )
    use_bf16 = torch.cuda.is_bf16_supported()
    training_args = SFTConfig(
        output_dir=str(args.output_dir),
        num_train_epochs=args.epochs,
        per_device_train_batch_size=1,
        per_device_eval_batch_size=1,
        gradient_accumulation_steps=args.gradient_accumulation,
        learning_rate=1e-4 if args.role == "interviewer" else 5e-5,
        # TRL 1.12 removed warmup_ratio from SFTConfig; keep this compatible
        # with the Colab/runtime package by using an explicit step count.
        warmup_steps=1,
        lr_scheduler_type="cosine",
        logging_steps=10,
        eval_strategy="steps",
        eval_steps=50,
        save_strategy="steps",
        save_steps=50,
        save_total_limit=2,
        bf16=use_bf16,
        fp16=not use_bf16,
        gradient_checkpointing=True,
        max_length=max_length,
        # Packing without flash-attention can cross-contaminate examples.
        packing=False,
        assistant_only_loss=assistant_only_loss,
        report_to="none",
        remove_unused_columns=False,
    )
    trainer = SFTTrainer(
        model=model,
        args=training_args,
        train_dataset=dataset["train"],
        eval_dataset=dataset["validation"],
        processing_class=tokenizer,
        peft_config=lora,
    )
    trainer.train(resume_from_checkpoint=args.resume_from_checkpoint)
    trainer.save_model(str(args.output_dir))
    tokenizer.save_pretrained(str(args.output_dir))
    metadata = {
        "base_model": model_id,
        "role": args.role,
        "method": "QLoRA",
        "quantization": "NF4-4bit-double-quant",
        "max_length": max_length,
        "assistant_only_loss": assistant_only_loss,
    }
    (args.output_dir / "training_metadata.json").write_text(
        json.dumps(metadata, indent=2) + "\n", encoding="utf-8"
    )
    print(json.dumps(metadata, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
