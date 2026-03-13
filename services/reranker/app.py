import os
from contextlib import asynccontextmanager
from typing import List

CACHE_DIR = os.getenv("RERANKER_CACHE_DIR", "/models/huggingface")
os.environ.setdefault("HF_HOME", CACHE_DIR)
os.environ.setdefault("SENTENCE_TRANSFORMERS_HOME", os.path.join(CACHE_DIR, "sentence-transformers"))

from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse
from pydantic import BaseModel, Field
from sentence_transformers import CrossEncoder


MODEL_NAME = os.getenv("RERANKER_MODEL", "BAAI/bge-reranker-v2-m3")
MAX_DOCS = int(os.getenv("RERANKER_MAX_DOCS", "50"))
MAX_CHARS = int(os.getenv("RERANKER_MAX_CHARS", "2400"))
BATCH_SIZE = int(os.getenv("RERANKER_BATCH_SIZE", "8"))


class Document(BaseModel):
    id: str = Field(min_length=1)
    text: str = Field(min_length=1)


class RerankRequest(BaseModel):
    query: str = Field(min_length=1)
    top_n: int = Field(default=10, ge=1, le=MAX_DOCS)
    documents: List[Document]


class RerankResult(BaseModel):
    id: str
    score: float


class RerankResponse(BaseModel):
    results: List[RerankResult]


state = {"model": None, "ready": False, "error": ""}


@asynccontextmanager
async def lifespan(_: FastAPI):
    try:
        model = CrossEncoder(MODEL_NAME, trust_remote_code=True)
        _ = model.predict([["warmup query", "warmup document"]], batch_size=1)
        state["model"] = model
        state["ready"] = True
        state["error"] = ""
    except Exception as exc:  # pragma: no cover
        state["model"] = None
        state["ready"] = False
        state["error"] = str(exc)
    yield


app = FastAPI(title="OneBook Reranker", version="1.0.0", lifespan=lifespan)


@app.get("/healthz")
def healthz():
    if not state["ready"] or state["model"] is None:
        return JSONResponse(status_code=503, content={"status": "not_ready", "error": state["error"]})
    return {
        "status": "ok",
        "model": MODEL_NAME,
        "cache_dir": CACHE_DIR,
        "max_docs": MAX_DOCS,
        "max_chars": MAX_CHARS,
    }


@app.post("/rerank", response_model=RerankResponse)
def rerank(request: RerankRequest):
    if not state["ready"] or state["model"] is None:
        raise HTTPException(status_code=503, detail="model not ready")
    if len(request.documents) == 0:
        return RerankResponse(results=[])
    if len(request.documents) > MAX_DOCS:
        raise HTTPException(status_code=400, detail=f"too many documents: {len(request.documents)} > {MAX_DOCS}")

    pairs = []
    ids = []
    for doc in request.documents:
        text = doc.text.strip()
        if not text:
            continue
        if len(text) > MAX_CHARS:
            text = text[:MAX_CHARS]
        ids.append(doc.id)
        pairs.append([request.query.strip(), text])

    if not pairs:
        return RerankResponse(results=[])

    scores = state["model"].predict(pairs, batch_size=BATCH_SIZE, show_progress_bar=False)
    ranked = sorted(
        (RerankResult(id=doc_id, score=float(score)) for doc_id, score in zip(ids, scores)),
        key=lambda item: item.score,
        reverse=True,
    )
    return RerankResponse(results=ranked[: request.top_n])


if __name__ == "__main__":  # pragma: no cover
    import uvicorn

    port = int(os.getenv("RERANKER_PORT", "8088"))
    uvicorn.run(app, host="0.0.0.0", port=port)
