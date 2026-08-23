FROM python:3.12-slim

WORKDIR /app

# The dependency list alone first, so a code change does not re-download
# fastembed's ONNX runtime. The stub package is what `pip install .`
# needs to exist; the real one is copied over it below.
COPY pyproject.toml ./
RUN mkdir knowdo_worker && touch knowdo_worker/__init__.py && pip install --no-cache-dir .

# Bake the embedding model into the image so the first job does not wait
# on a model download.
ARG EMBED_MODEL=BAAI/bge-small-en-v1.5
ENV EMBED_MODEL=$EMBED_MODEL
RUN python -c "from fastembed import TextEmbedding; TextEmbedding('$EMBED_MODEL')"

COPY knowdo_worker ./knowdo_worker

# The queue consumer. The `ai` service runs the same image with a
# uvicorn command instead.
CMD ["python", "-m", "knowdo_worker.consumer"]
