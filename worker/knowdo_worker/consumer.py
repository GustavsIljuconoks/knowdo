"""BLPOP loop over the ingest queue.

Run as `python -m knowdo_worker.consumer`. Separate from the FastAPI
service because ingestion is bursty and scales on queue depth, while
/ask is latency-sensitive and scales on request rate — a backlog of
PDFs must not slow answers down.
"""

import json
import logging
import os

import redis

from .ingest import ingest

# Must match queue.IngestList in the Go API.
INGEST_LIST = "knowdo:jobs:ingest"

# How long each BLPOP blocks server-side before returning nil.
BLOCK_SECONDS = 5

# The socket read deadline MUST outlast the block, or the client gives up
# before the server's reply arrives: Redis answers a 5s block at ~5.1s.
# redis-py 8 defaults socket_timeout to 5s, which makes the naive
# `blpop(list, timeout=5)` crash on the first idle iteration.
SOCKET_TIMEOUT = BLOCK_SECONDS * 2

log = logging.getLogger(__name__)


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")

    client = redis.from_url(os.environ["REDIS_URL"], socket_timeout=SOCKET_TIMEOUT)
    log.info("consuming %s", INGEST_LIST)

    while True:
        # A timeout rather than blocking forever, so SIGTERM from
        # `docker compose down` is acted on within five seconds.
        item = client.blpop(INGEST_LIST, timeout=BLOCK_SECONDS)

        if item is None:
            continue

        _, body = item

        try:
            document_id = json.loads(body)["document_id"]
        except (ValueError, KeyError, TypeError):
            log.exception("dropping malformed job: %r", body)
            continue

        try:
            chunk_count = ingest(document_id)
            log.info("document %s ingested into %d chunks", document_id, chunk_count)
        except Exception:
            # ingest() has already written status='failed' and the
            # message, which is what the user sees. This is only for us.
            log.exception("ingesting document %s", document_id)


if __name__ == "__main__":
    main()
