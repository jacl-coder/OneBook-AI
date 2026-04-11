FROM python:3.11-slim

COPY --from=ghcr.io/astral-sh/uv:0.9.27 /uv /uvx /bin/

# System deps for PaddleOCR (libGL, libglib2)
RUN apt-get update && apt-get install -y --no-install-recommends \
    libglib2.0-0 \
    libgl1 \
    libgomp1 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

ARG OCR_PADDLE_PACKAGE=paddlepaddle==3.0.0
ARG OCR_PADDLE_INDEX=https://www.paddlepaddle.org.cn/packages/stable/cpu/

ENV UV_LINK_MODE=copy \
    UV_COMPILE_BYTECODE=1 \
    PATH="/app/.venv/bin:$PATH"

COPY pyproject.toml uv.lock ./
RUN uv sync --locked --no-dev --no-install-project \
    && /app/.venv/bin/python -m ensurepip --upgrade \
    && /app/.venv/bin/python -m pip uninstall -y paddlepaddle paddlepaddle-gpu >/dev/null 2>&1 || true \
    && /app/.venv/bin/python -m pip install --no-cache-dir --timeout 180 --retries 10 -i "${OCR_PADDLE_INDEX}" "${OCR_PADDLE_PACKAGE}"

COPY app.py .

# Pre-warm models at build time so the first request is fast
# Set env to skip model source connectivity check
ENV PADDLE_PDX_DISABLE_MODEL_SOURCE_CHECK=True
RUN OCR_DEVICE=cpu OCR_ENABLE_HPI=false python -c "from paddleocr import PaddleOCR; PaddleOCR(device='cpu', use_doc_orientation_classify=False, use_doc_unwarping=False, use_textline_orientation=False)" || true

EXPOSE 8087

CMD ["python", "app.py"]
