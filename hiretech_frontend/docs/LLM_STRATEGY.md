# LLM Strategy, Router, and Dataset Plan

## 1. LLM architecture principle

The LLM provider must not be hard-coded into application logic. Each model must connect through an adapter.

Recommended layers:

- `ModelRegistry`: Stores available models.
- `ModelAdapter`: Abstracts provider-specific API differences.
- `TaskClassifier`: Determines task type and difficulty.
- `LLMRouter`: Selects the appropriate model.
- `FallbackPolicy`: Selects a backup model after an error or limit.
- `EvaluationLogger`: Records model, latency, token, and error data.

## 2. Two-LLM roles

### Model A — fast interviewer

- Real-time questions and answers
- Follow-up questions
- Simple technical explanations
- Low latency and low cost

### Model B — independent evaluator

- Code and explanation analysis
- System-design evaluation
- Rubric scoring
- Review of Model A’s assessment
- Final report generation

## 3. Initial model candidates

The final “fastest model” must be selected through measurements on the same task set, not through a theoretical list. Initial benchmark candidates:

- Google Gemini Flash-Lite: Fast, high-volume interaction.
- GPT-OSS or a similar open-weight model through Groq: Low-latency responses.
- Mistral Small: Instruction following, coding, reasoning, and structured output.
- A stronger Flash/mini-class model: Final evaluation.

Model IDs must be verified before release and remain configurable. They must not be embedded directly in the application code.

## 4. Router decision criteria

- Task type
- Prompt length
- Whether code is included
- Complexity level
- Data sensitivity
- Maximum acceptable latency
- Token cost
- Model health status
- Historical task success rate

## 5. Benchmark measurements

Every model must be tested against the same dataset:

- Time to first token
- Total latency
- Tokens per second
- Valid JSON rate
- Code-evaluation accuracy
- Agreement with human rubric scores
- Prompt-injection resistance
- PII leakage rate
- Cost per successful interview

## 6. Dataset packages

### Technical question dataset

- Question
- Position
- Seniority
- Technology
- Expected concepts
- Follow-up questions
- Difficulty level

### Coding tasks

- Task description
- Starter repository or files
- Programming language
- Public tests
- Hidden tests
- Expected behavior
- Edge-case list

### System-design dataset

- Scenario
- Functional requirements
- Scale assumptions
- Expected components
- Critical trade-offs
- Evaluation rubric

### Answer-scoring dataset

- Candidate answer
- Position
- Seniority
- Competency label
- Human score
- Rationale
- Error category

### Security dataset

- Prompt-injection examples
- Tool-poisoning examples
- PII-leakage examples
- Unauthorized action requests
- Malicious code comments
- Expected safe behavior

## 7. Dataset creation when public data is insufficient

Synthetic data may be created, but synthetic examples must not be treated as ground truth without review.

1. Prepare an expert rubric.
2. Generate draft questions and answers with an LLM.
3. Have a technical reviewer correct the examples.
4. Label the examples.
5. Split them into training, validation, and test sets.
6. Keep the test set out of fine-tuning.

## 8. Fine-tuning scope

The first phase should fine-tune a small model or adapter rather than training a foundation model from scratch.

Recommended tasks:

- Competency classification
- Answer-quality classification
- AI-usage-level classification
- Prompt-injection classification
- Categorization of report evidence

Fine-tuning versions must be stored together with the dataset version, model version, and evaluation results.

