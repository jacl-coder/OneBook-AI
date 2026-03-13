FROM python:3.11-slim

WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    RERANKER_CACHE_DIR=/models/huggingface \
    HF_HOME=/models/huggingface \
    SENTENCE_TRANSFORMERS_HOME=/models/huggingface/sentence-transformers

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app.py .

RUN mkdir -p /models/huggingface/sentence-transformers

CMD ["python", "app.py"]
