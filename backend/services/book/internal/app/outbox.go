package app

import (
	"context"
	"encoding/json"
	"time"

	"onebookai/internal/util"
	"onebookai/pkg/domain"
)

const (
	outboxTopicIngestEnqueue = "ingest.enqueue"
	outboxDispatchInterval   = 2 * time.Second
	outboxDispatchLease      = 30 * time.Second
)

type ingestOutboxPayload struct {
	BookID     string `json:"bookId"`
	Generation int64  `json:"generation,omitempty"`
}

func buildIngestOutboxMessage(bookID string, generation int64) *domain.OutboxMessage {
	payload, _ := json.Marshal(ingestOutboxPayload{
		BookID:     bookID,
		Generation: generation,
	})
	now := time.Now().UTC()
	return &domain.OutboxMessage{
		ID:           util.NewID(),
		Topic:        outboxTopicIngestEnqueue,
		ResourceType: "book",
		ResourceID:   bookID,
		PayloadJSON:  payload,
		AvailableAt:  now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func (a *App) startOutboxWorker() {
	go func() {
		ticker := time.NewTicker(outboxDispatchInterval)
		defer ticker.Stop()
		for range ticker.C {
			a.dispatchPendingIngestOutbox(context.Background(), 10)
		}
	}()
}

func (a *App) dispatchPendingIngestOutbox(ctx context.Context, limit int) {
	items, err := a.store.ClaimOutboxMessages(outboxTopicIngestEnqueue, limit, outboxDispatchLease)
	if err != nil {
		return
	}
	for _, item := range items {
		var payload ingestOutboxPayload
		if err := json.Unmarshal(item.PayloadJSON, &payload); err != nil {
			_ = a.store.ReleaseOutboxMessage(item.ID, err.Error(), time.Now().UTC().Add(time.Hour))
			continue
		}
		if err := a.ingest.Enqueue(payload.BookID, payload.Generation); err != nil {
			retryAfter := time.Now().UTC().Add(backoffForAttempt(item.Attempts))
			_ = a.store.ReleaseOutboxMessage(item.ID, err.Error(), retryAfter)
			continue
		}
		_ = a.store.MarkOutboxDispatched(item.ID)
	}
}

func backoffForAttempt(attempt int) time.Duration {
	if attempt <= 1 {
		return 2 * time.Second
	}
	delay := time.Duration(attempt*attempt) * time.Second
	if delay > 5*time.Minute {
		return 5 * time.Minute
	}
	return delay
}
