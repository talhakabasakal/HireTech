# HireTech QLoRA training

Bu klasör Go runtime'ından bağımsız, tekrarlanabilir offline eğitim akışıdır. İki rol için iki ayrı adapter üretilir:

- Interviewer: `microsoft/Phi-4-mini-instruct`
- Evaluator: `Qwen/Qwen3-4B`

## Akış

1. Yalnızca sentetik veya uzman tarafından onaylanmış örnekleri `data/raw/*.jsonl` altına koyun.
2. `scripts/prepare_datasets.py` ile kayıtları role göre ayırın, deduplicate edin ve sabit hash ile train/validation/test split oluşturun.
3. `scripts/validate_training_data.py` ile son assistant mesajını mevcut interviewer/evaluator JSON sözleşmesine doğrulatın.
4. Önce iki base modeli adaptersız değerlendirin; sonra her rolü ayrı QLoRA ile eğitin.
5. Hold-out, Türkçe, injection ve PII testlerini eğitim dışında tutun.

## Kurulum ve komutlar

GPU'lu bir makinede (CUDA + uygun bitsandbytes kurulumu):

```bash
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

python scripts/generate_seed_dataset.py \
  --output data/seed/hiretech-synthetic-seed.jsonl

python scripts/prepare_datasets.py \
  --input data/seed \
  --output data/curated \
  --manifest data/curated/manifest.json

python scripts/validate_training_data.py --role interviewer
python scripts/validate_training_data.py --role evaluator

python scripts/train_qlora.py \
  --role interviewer \
  --data-dir data/curated/interviewer \
  --output-dir artifacts/phi-4-mini-interviewer \
  --trust-remote-code

python scripts/train_qlora.py \
  --role evaluator \
  --data-dir data/curated/evaluator \
  --output-dir artifacts/qwen3-4b-evaluator
```

`Qwen3-4B` evaluator olarak kullanılacağı için değerlendirme örneklerinde mümkünse `/no_think` kullanın; evaluator çıktısı kanıt-temelli, kısa ve şema uyumlu olsun. Ham chain-of-thought, gerçek aday verisi, e-posta/telefon gibi PII ve nihai işe alım kararı eğitim verisine girmemelidir.

`data/seed` içindeki sentetik seed kayıtları generator ile yeniden üretilebilir. Harici kaynaklardan gelecek uzman onaylı kayıtlar için `data/raw/README.md` formatını kullanın. Sadece `approved=true`, `expert_reviewed=true`, `training_eligible=true` olan kayıtlar SFT'ye alınır; benchmark/security kayıtları otomatik dışlanır.
Hash tabanlı split küçük veri kümelerinde bir split'i boş bırakabilir; gerçek eğitimden önce manifestte her rol için train ve validation kayıtlarının dolu olduğunu kontrol edin.

## Donanım notu

Bu repo eğitim kodunu sağlar; model ağırlıklarını bu geliştirme ortamında üretmez. QLoRA 4-bit bellek kullanımını azaltır ancak GPU gerektirir. 12 GB VRAM'de önce `max_length=2048`, batch size 1 ve gradient accumulation ile küçük deneme çalıştırın; evaluator için 12–16 GB VRAM ile başlanabilir; daha uzun bağlamlarda 24 GB+ daha rahat olacaktır.
