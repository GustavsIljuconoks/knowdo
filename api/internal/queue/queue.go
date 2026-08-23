package queue

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

// IngestList is the Redis list the Python consumer BLPOPs from. The
// name is part of the cross-service contract; changing it here means
// changing it in worker/knowdo_worker/consumer.py too.
const IngestList = "knowdo:jobs:ingest"

// IngestJob is the payload the worker receives.
type IngestJob struct {
	DocumentID int64 `json:"document_id"`
}

func (j IngestJob) payload() ([]byte, error) {
	return json.Marshal(j)
}

// Queue is the enqueue boundary. Handlers depend on this interface so
// tests don't need a running Redis.
type Queue interface {
	EnqueueIngest(ctx context.Context, documentID int64) error
}

type Redis struct {
	client *redis.Client
}

func NewRedis(url string) (*Redis, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}

	return &Redis{client: redis.NewClient(opts)}, nil
}

func (q *Redis) EnqueueIngest(ctx context.Context, documentID int64) error {
	body, err := IngestJob{DocumentID: documentID}.payload()
	if err != nil {
		return err
	}

	// ponytail: plain LPUSH, no retry/DLQ. A dropped job leaves the
	// document at status 'pending' and is re-driven by re-uploading.
	// Add a reliable queue (BRPOPLPUSH + ack list) when a job is
	// actually observed to be lost.
	return q.client.LPush(ctx, IngestList, body).Err()
}

func (q *Redis) Ping(ctx context.Context) error {
	return q.client.Ping(ctx).Err()
}
