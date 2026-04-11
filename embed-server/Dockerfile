FROM python:3.11-slim

WORKDIR /app

COPY requirements.txt .
# fastembed 依赖 ONNX Runtime，无需 PyTorch/CUDA，镜像远比 sentence-transformers 小
RUN pip install --no-cache-dir -r requirements.txt

COPY main.py .

# ONNX 模型会在首次启动时从 HuggingFace 下载并缓存
# 可通过挂载 volume 到 /root/.cache/huggingface 避免重复下载
ENV EMBEDDING_MODEL=intfloat/multilingual-e5-large
ENV PORT=8001
ENV BATCH_SIZE=32

EXPOSE 8001

CMD ["python", "main.py"]
