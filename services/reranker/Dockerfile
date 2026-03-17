FROM python:3.11-slim

COPY --from=ghcr.io/astral-sh/uv:0.9.27 /uv /uvx /bin/

WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    UV_LINK_MODE=copy \
    UV_COMPILE_BYTECODE=1 \
    UV_HTTP_TIMEOUT=120 \
    RERANKER_CACHE_DIR=/models/huggingface \
    HF_HOME=/models/huggingface \
    SENTENCE_TRANSFORMERS_HOME=/models/huggingface/sentence-transformers \
    PATH="/app/.venv/bin:$PATH"

COPY pyproject.toml ./
COPY app.py ./

RUN mkdir -p /models/huggingface/sentence-transformers

RUN uv venv

RUN . /app/.venv/bin/activate && \
    uv pip install --index-url https://download.pytorch.org/whl/cpu torch

RUN . /app/.venv/bin/activate && \
    uv pip install fastapi==0.116.1 "uvicorn[standard]==0.35.0" "sentence-transformers==5.1.0"

CMD ["python", "app.py"]