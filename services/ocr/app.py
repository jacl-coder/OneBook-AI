"""
OneBook AI — OCR Service
Wraps PaddleOCR as an HTTP endpoint for the ingest service.

POST /ocr
  Content-Type: multipart/form-data
  Body: file=<pdf or image bytes>

Response 200:
  {
    "pages": [
      {"page": 1, "text": "...", "avg_score": 0.95},
      ...
    ]
  }

GET /healthz → {"status":"ok"}
"""

import os
import json
import tempfile
import logging
from typing import List

from fastapi import FastAPI, File, UploadFile, HTTPException
from fastapi.responses import JSONResponse
from pydantic import BaseModel
from paddleocr import PaddleOCR


class UTF8JSONResponse(JSONResponse):
    """JSONResponse that never ASCII-escapes Unicode characters."""
    def render(self, content) -> bytes:
        return json.dumps(
            content, ensure_ascii=False,
            allow_nan=False, separators=(",", ":"),
        ).encode("utf-8")

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("ocr-service")

app = FastAPI(title="OneBook OCR Service", version="1.0.0", default_response_class=UTF8JSONResponse)

# Initialise once at startup — model files are cached in /root/.paddlex
_ocr: PaddleOCR | None = None


def get_ocr() -> PaddleOCR:
    global _ocr
    if _ocr is None:
        logger.info("Initialising PaddleOCR (first call, may download models)…")
        _ocr = PaddleOCR(
            use_doc_orientation_classify=False,
            use_doc_unwarping=False,
            use_textline_orientation=False,
        )
        logger.info("PaddleOCR ready.")
    return _ocr


class OCRPage(BaseModel):
    page: int
    text: str
    avg_score: float


class OCRResponse(BaseModel):
    pages: List[OCRPage]


@app.get("/healthz")
def healthz():
    return {"status": "ok"}


@app.post("/ocr", response_model=OCRResponse)
async def ocr_file(file: UploadFile = File(...)):
    suffix = os.path.splitext(file.filename or "upload.pdf")[1].lower()
    if suffix not in {".pdf", ".jpg", ".jpeg", ".png", ".bmp"}:
        raise HTTPException(status_code=400, detail=f"Unsupported file type: {suffix}")

    data = await file.read()
    if not data:
        raise HTTPException(status_code=400, detail="Empty file")

    with tempfile.NamedTemporaryFile(suffix=suffix, delete=False) as tmp:
        tmp.write(data)
        tmp_path = tmp.name

    try:
        ocr = get_ocr()
        result = ocr.predict(tmp_path)
        pages = _parse_result(result)
        if not pages:
            raise HTTPException(status_code=422, detail="No text extracted from file")
        return OCRResponse(pages=pages)
    except HTTPException:
        raise
    except Exception as exc:
        logger.exception("OCR processing failed: %s", exc)
        raise HTTPException(status_code=500, detail=str(exc))
    finally:
        os.unlink(tmp_path)


def _parse_result(result) -> List[OCRPage]:
    """Convert PaddleOCR predict() output to OCRPage list."""
    pages: List[OCRPage] = []

    if result is None:
        return pages

    # result may be a generator or list of per-page dicts
    items = list(result)

    for idx, item in enumerate(items):
        page_num = idx + 1
        texts: list[str] = []
        scores: list[float] = []

        # PaddleOCR v3.x predict() returns dicts with rec_texts / rec_scores flat lists.
        # Older objects with attribute-based access are also handled as fallback.
        if isinstance(item, dict):
            pi = item.get("page_index")
            if pi is not None:
                page_num = int(pi) + 1
            if "rec_texts" in item:
                # v3.x format
                for t, s in zip(item.get("rec_texts") or [], item.get("rec_scores") or []):
                    if t and str(t).strip():
                        texts.append(str(t).strip())
                        scores.append(float(s) if s else 0.0)
            else:
                # v2.x fallback: rec_res is a list of (text, score) tuples
                for box_info in item.get("rec_res", []):
                    t = box_info[0] if isinstance(box_info, (list, tuple)) else box_info.get("text", "")
                    s = box_info[1] if isinstance(box_info, (list, tuple)) else box_info.get("score", 0.0)
                    if t and str(t).strip():
                        texts.append(str(t).strip())
                        scores.append(float(s) if s else 0.0)
        elif hasattr(item, "rec_texts"):
            # object-style access (some paddleocr versions)
            for t, s in zip(item.rec_texts or [], item.rec_scores or []):
                if t and str(t).strip():
                    texts.append(str(t).strip())
                    scores.append(float(s) if s else 0.0)

        if not texts:
            continue

        avg_score = sum(scores) / len(scores) if scores else 0.0
        pages.append(OCRPage(
            page=page_num,
            text="\n".join(texts),
            avg_score=round(avg_score, 4),
        ))

    return pages


if __name__ == "__main__":
    import uvicorn
    port = int(os.getenv("OCR_PORT", "8087"))
    uvicorn.run("app:app", host="0.0.0.0", port=port, log_level="info")
