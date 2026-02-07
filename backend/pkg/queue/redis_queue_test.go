package queue

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisJobQueueRequeueAndAckSuccess(t *testing.T) {
	q, ctx, msgID, jobID, bookID := newPendingQueueMessage(t)

	if err := q.requeueAndAck(ctx, msgID, jobID, bookID); err != nil {
		t.Fatalf("requeue and ack: %v", err)
	}

	pending, err := q.client.XPending(ctx, q.stream, q.group).Result()
	if err != nil {
		t.Fatalf("xpending: %v", err)
	}
	if pending.Count != 0 {
		t.Fatalf("expected no pending messages, got %d", pending.Count)
	}

	streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    q.group,
		Consumer: "consumer-2",
		Streams:  []string{q.stream, ">"},
		Count:    1,
		Block:    0,
	}).Result()
	if err != nil {
		t.Fatalf("read requeued message: %v", err)
	}
	if len(streams) != 1 || len(streams[0].Messages) != 1 {
		t.Fatalf("expected one requeued message, got %+v", streams)
	}
	got := streams[0].Messages[0]
	if got.Values["job_id"] != jobID || got.Values["book_id"] != bookID {
		t.Fatalf("unexpected requeued payload: %+v", got.Values)
	}
}

func TestRedisJobQueueRequeueAndAckFailureKeepsPendingMessage(t *testing.T) {
	q, ctx, msgID, jobID, bookID := newPendingQueueMessage(t)

	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	if err := q.requeueAndAck(canceledCtx, msgID, jobID, bookID); err == nil {
		t.Fatalf("expected requeueAndAck to fail on canceled context")
	}

	pending, err := q.client.XPending(ctx, q.stream, q.group).Result()
	if err != nil {
		t.Fatalf("xpending: %v", err)
	}
	if pending.Count != 1 {
		t.Fatalf("expected original message to remain pending, got %d", pending.Count)
	}

	streamLen, err := q.client.XLen(ctx, q.stream).Result()
	if err != nil {
		t.Fatalf("xlen: %v", err)
	}
	if streamLen != 1 {
		t.Fatalf("expected no new message in stream on failure, got len=%d", streamLen)
	}
}

func newPendingQueueMessage(t *testing.T) (*RedisJobQueue, context.Context, string, string, string) {
	t.Helper()

	redisSrv := miniredis.RunT(t)
	q, err := NewRedisJobQueue(RedisQueueConfig{
		Addr:       redisSrv.Addr(),
		Stream:     "test:queue",
		Group:      "test-group",
		Consumer:   "consumer-1",
		RetryDelay: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new queue: %v", err)
	}

	ctx := context.Background()
	q.ensureGroup(ctx)

	job, err := q.Enqueue(ctx, "book-1")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    q.group,
		Consumer: "consumer-1",
		Streams:  []string{q.stream, ">"},
		Count:    1,
		Block:    0,
	}).Result()
	if err != nil {
		t.Fatalf("readgroup: %v", err)
	}
	if len(streams) != 1 || len(streams[0].Messages) != 1 {
		t.Fatalf("expected one pending message, got %+v", streams)
	}

	msg := streams[0].Messages[0]
	return q, ctx, msg.ID, job.ID, job.BookID
}
