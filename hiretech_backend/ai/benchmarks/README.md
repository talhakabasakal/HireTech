# AI Benchmark Runner

`run_benchmark.py` is a bounded, contract-first runner for the OpenAI-compatible
interviewer and evaluator endpoints. It uses the frozen curated `test.jsonl`
splits, verifies their SHA-256 manifest entry before execution, sends deterministic
requests (`temperature=0`), and writes only metadata and aggregate evidence. Raw
prompts and model output are never written to the report.

The runner measures:

- first-pass JSON parse and JSON Schema validity;
- prohibited hiring-decision output;
- client-observed and server-reported completion latency;
- prompt, completion, and total token usage.

It deliberately does not manufacture quality, bias, evidence-faithfulness, or
security results. Those metrics require blinded expert review, approved labels,
and dedicated security/provider-failure suites. A report is evidence for review,
not release or model-promotion authority.

## Validate the frozen input without an endpoint

From `hiretech_backend`:

```bash
python3 ai/benchmarks/run_benchmark.py \
  --role interviewer \
  --dataset ai/training/data/curated/interviewer/test.jsonl \
  --validate-only
```

## Run against a reachable model endpoint

The endpoint must be explicitly supplied. The API key is read from an environment
variable and is never printed:

```bash
AI_BENCHMARK_API_KEY='redacted-in-shell-history-by-your-secret-manager' \
python3 ai/benchmarks/run_benchmark.py \
  --role interviewer \
  --dataset ai/training/data/curated/interviewer/test.jsonl \
  --endpoint http://127.0.0.1:8001/v1 \
  --model microsoft/Phi-4-mini-instruct \
  --registry-version registry-entry-version \
  --prompt-version prompt-version \
  --output /tmp/hiretech-interviewer-benchmark.json
```

For the evaluator, use the evaluator test split and the exact model served by the
evaluator endpoint. Use `--max-attempts 1` for a no-retry baseline or at most `3`
for the bounded retry comparison required by the promotion procedure.

External or expert-reviewed datasets must provide `--expected-sha256`; the runner
refuses an unpinned dataset. Keep hidden test sets outside shared logs and do not
tune prompts or thresholds using them.
