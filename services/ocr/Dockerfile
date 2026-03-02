FROM python:3.11-slim

# System deps for PaddleOCR (libGL, libglib2)
RUN apt-get update && apt-get install -y --no-install-recommends \
    libglib2.0-0 \
    libgl1 \
    libgomp1 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app.py .

# Pre-warm models at build time so the first request is fast
# Set env to skip model source connectivity check
ENV PADDLE_PDX_DISABLE_MODEL_SOURCE_CHECK=True
RUN python -c "from paddleocr import PaddleOCR; PaddleOCR(use_doc_orientation_classify=False, use_doc_unwarping=False, use_textline_orientation=False)" || true

EXPOSE 8087

CMD ["python", "app.py"]
