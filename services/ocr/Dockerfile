FROM python:3.11-slim

COPY --from=ghcr.io/astral-sh/uv:0.9.27 /uv /uvx /bin/

# System deps for PaddleOCR (libGL, libglib2)
RUN apt-get update && apt-get install -y --no-install-recommends \
    libglib2.0-0 \
    libgl1 \
    libgomp1 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

ARG OCR_PADDLE_PACKAGE=paddlepaddle-gpu==3.3.0
ARG OCR_PADDLE_INDEX=https://www.paddlepaddle.org.cn/packages/stable/cu126/

ENV UV_LINK_MODE=copy \
    UV_COMPILE_BYTECODE=1 \
    PATH="/app/.venv/bin:$PATH"

COPY pyproject.toml uv.lock ./
RUN uv sync --locked --no-dev --no-install-project \
    && /app/.venv/bin/python -m ensurepip --upgrade \
    && (/app/.venv/bin/python -m pip uninstall -y paddlepaddle paddlepaddle-gpu >/dev/null 2>&1 || true) \
    && /app/.venv/bin/python -m pip install --no-cache-dir --timeout 180 --retries 10 -i "${OCR_PADDLE_INDEX}" "${OCR_PADDLE_PACKAGE}"

COPY app.py .

# Validate installed package versions. GPU imports and model initialisation happen at runtime.
ENV PADDLE_PDX_DISABLE_MODEL_SOURCE_CHECK=True
RUN python -c "import importlib.metadata as md; print('paddlepaddle-gpu', md.version('paddlepaddle-gpu')); print('paddleocr', md.version('paddleocr'))"

EXPOSE 8087

CMD ["python", "app.py"]
