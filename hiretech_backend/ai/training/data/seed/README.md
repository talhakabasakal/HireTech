# HireTech synthetic seed dataset

`hiretech-synthetic-seed.jsonl` ilk QLoRA smoke-test setidir. İçerik sentetiktir; gerçek aday verisi, kişisel veri, provider payload'ı veya ham chain-of-thought içermez.

- Interviewer kayıtları `microsoft/Phi-4-mini-instruct` için soru sorma ve güvenli interview akışını hedefler.
- Evaluator kayıtları `Qwen/Qwen3-4B` için evidence, rubric, confidence ve human-review davranışını hedefler.
- Bu set üretim kalitesinde büyük bir corpus değildir; kaynak datasetlerden uzman onaylı dönüşümler geldikçe büyütülmelidir.

Dataset, ilgili JSON output schema'larına karşı doğrulanmadan eğitime alınmamalıdır.
