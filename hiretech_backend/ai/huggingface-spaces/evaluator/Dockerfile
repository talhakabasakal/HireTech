FROM nvidia/cuda:12.4.1-runtime-ubuntu22.04

ENV DEBIAN_FRONTEND=noninteractive \
    PIP_NO_CACHE_DIR=1 \
    PYTHONUNBUFFERED=1 \
    AI_ROLE=evaluator \
    AI_MODEL_ID=Qwen/Qwen3-4B \
    AI_ADAPTER_ID=TalhaKa/hiretech-evaluator-qlora \
    AI_HOST=0.0.0.0 \
    AI_PORT=8000

RUN apt-get update \
    && apt-get install -y --no-install-recommends python3 python3-pip \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY requirements.txt /app/requirements.txt
RUN python3 -m pip install --upgrade pip \
    && python3 -m pip install -r /app/requirements.txt

COPY server.py /app/server.py
COPY contract_runtime.py /app/contract_runtime.py
CMD ["python3", "/app/server.py"]
