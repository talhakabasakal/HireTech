# HireTech Hugging Face Spaces

These two Docker Spaces expose the fine-tuned adapters through the same
OpenAI-compatible `/v1/chat/completions` contract used by the Go backend.

- `interviewer`: `TalhaKa/hiretech-interviewer-qlora` on top of
  `microsoft/Phi-4-mini-instruct`
- `evaluator`: `TalhaKa/hiretech-evaluator-qlora` on top of `Qwen/Qwen3-4B`

The Space secret `AI_API_KEY` must be set to a random service key. The Hub
token is only used as `HF_TOKEN` if a model repository is private; it is not
returned by the API or sent to callers.
