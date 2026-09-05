# HireTech QLoRA — Google Colab

Bu paket, iki rol adaptörünü Colab GPU üzerinde eğitmek için hazırlanmıştır:

- Interviewer: `microsoft/Phi-4-mini-instruct`
- Evaluator: `Qwen/Qwen3-4B`

> Bu arşivdeki veri seti şu an 24 kayıtlık sentetik seed/smoke-test verisidir. Eğitim komutunun çalıştığını doğrular; gerçek model kalitesi için daha sonra lisanslı ve anonimleştirilmiş veriyle büyütülmelidir.

## Colab adımları

1. Colab'da `Runtime > Change runtime type > T4 GPU` seçin.
2. Aşağıdaki hücreleri sırayla çalıştırın:

```python
from google.colab import files
uploaded = files.upload()  # hiretech-qlora-colab-package.zip seçin
```

```python
!rm -rf /content/hiretech-training
!mkdir -p /content/hiretech-training
!unzip -q hiretech-qlora-colab-package.zip -d /content/hiretech-training
%cd /content/hiretech-training
!pip install -q -U "transformers>=4.51" "datasets>=3.0" "accelerate>=1.0" "bitsandbytes>=0.45" "peft>=0.14" "trl>=0.15" "sentencepiece>=0.2" "jsonschema>=4.23"
!nvidia-smi
```

3. Veriyi kontrol edin:

```python
!python scripts/validate_training_data.py --role interviewer --data-dir data/curated/interviewer
!python scripts/validate_training_data.py --role evaluator --data-dir data/curated/evaluator
```

4. Önce iki kısa smoke-test eğitimini başlatın:

```python
!python scripts/train_qlora.py --role interviewer --data-dir data/curated/interviewer --output-dir artifacts/phi-4-mini-interviewer --trust-remote-code --epochs 2 --gradient-accumulation 4
```

```python
!python scripts/train_qlora.py --role evaluator --data-dir data/curated/evaluator --output-dir artifacts/qwen3-4b-evaluator --epochs 2 --gradient-accumulation 4
```

5. Çıktıları cihazınıza indirin:

```python
!zip -qr /content/hiretech-qlora-adapters.zip artifacts
from google.colab import files
files.download('/content/hiretech-qlora-adapters.zip')
```

## Notlar

- İlk model indirmeleri büyük olabilir; Colab oturumunun açık kalması gerekir.
- Hugging Face erişim hatası olursa `from huggingface_hub import login; login()` ile token girin.
- OOM durumunda `--max-length 1024`, daha düşük `--gradient-accumulation` veya T4 yerine daha büyük GPU kullanın.
- Smoke-test başarılı olduktan sonra gerçek veri setini aynı `train.jsonl` ve `validation.jsonl` şemasıyla değiştirip eğitimi tekrarlayın.
