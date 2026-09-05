# HireTech local AI runtime

The Go backend uses two OpenAI-compatible local endpoints:

- Interviewer: `microsoft/Phi-4-mini-instruct` + `adapters/interviewer`
- Evaluator: `Qwen/Qwen3-4B` + `adapters/evaluator`

The adapter files are intentionally ignored by git. Keep the downloaded
`hiretech-qlora-adapters.zip` outside version control, extract its `artifacts`
contents into this directory, and keep the base model in the Hugging Face
cache. The base model weights are several gigabytes and are downloaded once.

```bash
cd hiretech_backend/ai/runtime
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
./serve_local_models.sh
```

In a second terminal, run the Go backend with:

```bash
AI_ENABLED=true \
AI_INTERVIEWER_MODEL_VERSION=qlora-smoketest-v1 \
AI_EVALUATOR_MODEL_VERSION=qlora-smoketest-v1 \
./dev.sh server
```

The runtime requires a CUDA GPU. For CPU-only development, leave
`AI_ENABLED=false` and the backend keeps its deterministic evaluation baseline.
The adapter is a smoke-test trained on the current synthetic seed dataset;
replace it with the enlarged, reviewed dataset before production use.
