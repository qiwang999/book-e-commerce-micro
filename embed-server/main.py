"""
本地 Embedding 服务器，兼容 OpenAI /v1/embeddings API。
使用 fastembed（ONNX Runtime），无需 PyTorch/CUDA，镜像小启动快。

默认模型：intfloat/multilingual-e5-large（1024 维，支持中英文多语言）

E5 前缀说明：
  Go 侧调用前已添加前缀，本服务按原文嵌入，不自动添加：
  - 书籍文本（passage）：Go 侧已加 "passage: " 前缀
  - 检索 query：   Go 侧已加 "query: "  前缀
"""

import os
import logging
import time
from typing import List, Union, Optional

from fastembed import TextEmbedding
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger(__name__)

MODEL_NAME = os.getenv("EMBEDDING_MODEL", "intfloat/multilingual-e5-large")
BATCH_SIZE = int(os.getenv("BATCH_SIZE", "32"))

app = FastAPI(title="BookHive Embedding Server", version="2.0.0")
model: Optional[TextEmbedding] = None


@app.on_event("startup")
async def startup_event():
    global model
    logger.info(f"Loading fastembed model: {MODEL_NAME}")
    t0 = time.time()
    model = TextEmbedding(model_name=MODEL_NAME)
    # 触发一次 warmup，确保 ONNX session 已初始化
    _ = list(model.embed(["warmup"]))
    logger.info(f"Model ready in {time.time()-t0:.1f}s")


# ─── OpenAI-compatible request / response schemas ─────────────────────────────

class EmbeddingRequest(BaseModel):
    input: Union[List[str], str]
    model: Optional[str] = None
    encoding_format: Optional[str] = "float"


class EmbeddingData(BaseModel):
    object: str = "embedding"
    embedding: List[float]
    index: int


class UsageInfo(BaseModel):
    prompt_tokens: int = 0
    total_tokens: int = 0


class EmbeddingResponse(BaseModel):
    object: str = "list"
    data: List[EmbeddingData]
    model: str
    usage: UsageInfo = UsageInfo()


# ─── Endpoints ────────────────────────────────────────────────────────────────

@app.post("/v1/embeddings", response_model=EmbeddingResponse)
def create_embeddings(req: EmbeddingRequest):
    if model is None:
        raise HTTPException(status_code=503, detail="Model not loaded yet, please retry")

    texts: List[str] = req.input if isinstance(req.input, list) else [req.input]
    if not texts:
        return EmbeddingResponse(data=[], model=MODEL_NAME)

    embeddings = list(model.embed(texts, batch_size=BATCH_SIZE))

    data = [
        EmbeddingData(embedding=emb.tolist(), index=i)
        for i, emb in enumerate(embeddings)
    ]
    return EmbeddingResponse(data=data, model=MODEL_NAME)


@app.get("/health")
def health():
    if model is None:
        return {"status": "loading", "model": MODEL_NAME}
    return {"status": "ok", "model": MODEL_NAME, "dim": 1024}


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=int(os.getenv("PORT", "8001")))
