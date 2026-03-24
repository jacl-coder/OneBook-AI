package queue

import (
	"context"
	"encoding/json"
	"errors"
)

// JobQueue defines queue operations used by async services.
type JobQueue interface {
	Enqueue(ctx context.Context, resourceID string) (JobStatus, error)
	EnqueueWithPayload(ctx context.Context, resourceID string, payload json.RawMessage) (JobStatus, error)
	GetJob(ctx context.Context, jobID string) (JobStatus, bool, error)
	Start(ctx context.Context, concurrency int, handler func(context.Context, JobStatus) error)
	Ready(ctx context.Context) error
}

// ErrJobTerminal indicates the message references a terminal job and should be acknowledged.
var ErrJobTerminal = errors.New("job already terminal")

type jobStore interface {
	CreateOrGetActiveJob(ctx context.Context, jobType, resourceType, resourceID string, payload json.RawMessage) (JobStatus, bool, error)
	GetJob(ctx context.Context, jobID string) (JobStatus, bool, error)
	MarkProcessing(ctx context.Context, jobID string) (JobStatus, error)
	MarkQueued(ctx context.Context, jobID, errMsg string) error
	MarkDone(ctx context.Context, jobID string) error
	MarkFailed(ctx context.Context, jobID, errMsg string) error
}

func defaultPayload(resourceType, resourceID string) (json.RawMessage, error) {
	body := map[string]string{
		resourceType + "Id": resourceID,
	}
	return json.Marshal(body)
}
