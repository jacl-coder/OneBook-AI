import asyncio
import os
from contextlib import asynccontextmanager
from typing import List, Optional

CACHE_DIR = os.getenv("RERANKER_CACHE_DIR", "/models/huggingface")
os.environ.setdefault("HF_HOME", CACHE_DIR)
os.environ.setdefault(
    "SENTENCE_TRANSFORMERS_HOME",
    os.path.join(CACHE_DIR, "sentence-transformers"),
)

from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse
from pydantic import BaseModel, Field
from sentence_transformers import CrossEncoder


MODEL_NAME = os.getenv("RERANKER_MODEL", "BAAI/bge-reranker-v2-m3")
MODEL_REVISION = os.getenv("RERANKER_MODEL_REVISION", "").strip()
MAX_DOCS = int(os.getenv("RERANKER_MAX_DOCS", "50"))
MAX_CHARS = int(os.getenv("RERANKER_MAX_CHARS", "2400"))
BATCH_SIZE = int(os.getenv("RERANKER_BATCH_SIZE", "8"))
ENABLE_WARMUP = os.getenv("RERANKER_ENABLE_WARMUP", "true").lower() in {"1", "true", "yes", "on"}


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


state = {
    "model": None,          # type: Optional[CrossEncoder]
    "ready": False,
    "loading": False,
    "error": "",
    "stage": "init",
}


def log(message: str) -> None:
    print(f"[reranker] {message}", flush=True)


def load_model_sync() -> None:
    """
    在后台线程中同步加载模型，避免阻塞 FastAPI startup。
    """
    if state["loading"]:
        log("load request ignored: already loading")
        return

    state["loading"] = True
    state["ready"] = False
    state["error"] = ""
    state["stage"] = "loading"

    try:
        log(f"cache_dir={CACHE_DIR}")
        log(f"model_name={MODEL_NAME}")
        log(f"model_revision={MODEL_REVISION or 'default'}")
        log("stage=build_model_kwargs")

        model_kwargs = {"trust_remote_code": True}
        if MODEL_REVISION:
            model_kwargs["revision"] = MODEL_REVISION

        state["stage"] = "loading_model"
        log("stage=loading_model begin")
        model = CrossEncoder(MODEL_NAME, **model_kwargs)
        log("stage=loading_model done")

        if ENABLE_WARMUP:
            state["stage"] = "warmup"
            log("stage=warmup begin")
            _ = model.predict(
                [["warmup query", "warmup document"]],
                batch_size=1,
                show_progress_bar=False,
            )
            log("stage=warmup done")
        else:
            log("warmup skipped by env RERANKER_ENABLE_WARMUP=false")

        state["model"] = model
        state["ready"] = True
        state["error"] = ""
        state["stage"] = "ready"
        log("stage=ready")
    except Exception as exc:
        state["model"] = None
        state["ready"] = False
        state["error"] = repr(exc)
        state["stage"] = "failed"
        log(f"stage=failed error={repr(exc)}")
    finally:
        state["loading"] = False


async def load_model_async() -> None:
    """
    异步包装：把同步模型加载放到线程里执行。
    """
    await asyncio.to_thread(load_model_sync)


@asynccontextmanager
async def lifespan(_: FastAPI):
    log("lifespan startup begin")
    asyncio.create_task(load_model_async())
    log("lifespan startup complete (model loading in background)")
    yield
    log("lifespan shutdown begin")
    state["model"] = None
    state["ready"] = False
    state["loading"] = False
    state["stage"] = "shutdown"
    log("lifespan shutdown complete")


app = FastAPI(
    title="OneBook Reranker",
    version="1.1.0",
    lifespan=lifespan,
)


@app.get("/healthz")
def healthz():
    if state["ready"] and state["model"] is not None:
        return {
            "status": "ok",
            "stage": state["stage"],
            "model": MODEL_NAME,
            "revision": MODEL_REVISION,
            "cache_dir": CACHE_DIR,
            "max_docs": MAX_DOCS,
            "max_chars": MAX_CHARS,
            "batch_size": BATCH_SIZE,
            "warmup_enabled": ENABLE_WARMUP,
        }

    return JSONResponse(
        status_code=503,
        content={
            "status": "not_ready",
            "stage": state["stage"],
            "loading": state["loading"],
            "error": state["error"],
            "model": MODEL_NAME,
            "revision": MODEL_REVISION,
            "cache_dir": CACHE_DIR,
        },
    )


@app.post("/rerank", response_model=RerankResponse)
def rerank(request: RerankRequest):
    if not state["ready"] or state["model"] is None:
        raise HTTPException(
            status_code=503,
            detail={
                "message": "model not ready",
                "stage": state["stage"],
                "error": state["error"],
            },
        )

    if len(request.documents) == 0:
        return RerankResponse(results=[])

    if len(request.documents) > MAX_DOCS:
        raise HTTPException(
            status_code=400,
            detail=f"too many documents: {len(request.documents)} > {MAX_DOCS}",
        )

    query = request.query.strip()
    if not query:
        raise HTTPException(status_code=400, detail="query is empty after stripping")

    pairs = []
    ids = []

    for doc in request.documents:
        text = doc.text.strip()
        if not text:
            continue

        if len(text) > MAX_CHARS:
            text = text[:MAX_CHARS]

        ids.append(doc.id)
        pairs.append([query, text])

    if not pairs:
        return RerankResponse(results=[])

    try:
        scores = state["model"].predict(
            pairs,
            batch_size=BATCH_SIZE,
            show_progress_bar=False,
        )
    except Exception as exc:
        raise HTTPException(
            status_code=500,
            detail=f"rerank inference failed: {repr(exc)}",
        ) from exc

    ranked = sorted(
        (
            RerankResult(id=doc_id, score=float(score))
            for doc_id, score in zip(ids, scores)
        ),
        key=lambda item: item.score,
        reverse=True,
    )

    return RerankResponse(results=ranked[: request.top_n])


if __name__ == "__main__":  # pragma: no cover
    import uvicorn

    port = int(os.getenv("RERANKER_PORT", "8088"))
    uvicorn.run(app, host="0.0.0.0", port=port)