# Raw canonical JSONL

Her satır tek bir, sentetik ve insan tarafından gözden geçirilmiş eğitim örneğidir. Kaynak dataset'leri doğrudan modele vermeyin; önce bu biçime dönüştürüp filtreleyin.

```json
{"record_id":"demo-001","source_id":"internal-synthetic-v1","role":"INTERVIEWER","approved":true,"expert_reviewed":true,"training_eligible":true,"security_case":false,"benchmark_only":false,"messages":[{"role":"system","content":"You are the HireTech interviewer. Return only the contract JSON."},{"role":"user","content":"Adaya Türkçe bir debugging sorusu sor."},{"role":"assistant","content":"{...interviewer-output.schema.json içindeki tam ve geçerli nesne...}"}]}
```

`messages` içindeki son mesaj `assistant` olmalı ve JSON olarak parse edilebilmelidir. Assistant içeriği ilgili sözleşmeye tam uymalıdır. Gerçek aday verisi, ham chain-of-thought, güvenlik saldırı örnekleri ve benchmark testleri bu klasöre eğitim kaydı olarak konulmamalıdır.
