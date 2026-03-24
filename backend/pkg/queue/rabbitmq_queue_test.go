package queue

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestDecodeJobEnvelopeRejectsMissingJobID(t *testing.T) {
	_, err := DecodeJobEnvelope([]byte(`{"jobType":"ingest","resourceType":"book","resourceId":"book-1"}`))
	if err == nil {
		t.Fatalf("DecodeJobEnvelope() expected error for missing jobId")
	}
}

func TestRabbitMQQueueEnqueueDeduplicatesActiveResource(t *testing.T) {
	store := newFakeJobStore()
	queue, err := NewRabbitMQJobQueue(RabbitMQQueueConfig{
		URL:          "amqp://guest:guest@localhost:5672/",
		Exchange:     "onebook.jobs",
		QueueName:    "onebook.ingest.jobs",
		ConsumerName: "onebook-ingest-service",
		JobType:      "ingest",
		ResourceType: "book",
		MaxRetries:   3,
		RetryDelay:   time.Millisecond,
		Store:        store,
	})
	if err != nil {
		t.Fatalf("NewRabbitMQJobQueue() error = %v", err)
	}
	published := 0
	queue.publishFn = func(context.Context, string, JobEnvelope) error {
		published++
		return nil
	}

	job1, err := queue.Enqueue(context.Background(), "book-1")
	if err != nil {
		t.Fatalf("Enqueue() first error = %v", err)
	}
	job2, err := queue.Enqueue(context.Background(), "book-1")
	if err != nil {
		t.Fatalf("Enqueue() second error = %v", err)
	}
	if job1.ID != job2.ID {
		t.Fatalf("Enqueue() job dedupe mismatch: %q != %q", job1.ID, job2.ID)
	}
	if published != 1 {
		t.Fatalf("publish count = %d, want 1", published)
	}
}

func TestRabbitMQQueueHandleProcessingErrorRequeues(t *testing.T) {
	store := newFakeJobStore()
	job, _, err := store.CreateOrGetActiveJob(context.Background(), "ingest", "book", "book-1", mustJSON(t, map[string]string{"bookId": "book-1"}))
	if err != nil {
		t.Fatalf("CreateOrGetActiveJob() error = %v", err)
	}
	job.Attempts = 1
	store.jobs[job.ID] = job

	queue, err := NewRabbitMQJobQueue(RabbitMQQueueConfig{
		URL:          "amqp://guest:guest@localhost:5672/",
		Exchange:     "onebook.jobs",
		QueueName:    "onebook.ingest.jobs",
		ConsumerName: "onebook-ingest-service",
		JobType:      "ingest",
		ResourceType: "book",
		MaxRetries:   3,
		RetryDelay:   0,
		Store:        store,
	})
	if err != nil {
		t.Fatalf("NewRabbitMQJobQueue() error = %v", err)
	}
	disposition, err := queue.handleProcessingError(context.Background(), job, context.DeadlineExceeded)
	if err != nil {
		t.Fatalf("handleProcessingError() error = %v", err)
	}
	if disposition != failureRequeue {
		t.Fatalf("disposition = %v, want %v", disposition, failureRequeue)
	}
	updated := store.jobs[job.ID]
	if updated.Status != StatusQueued {
		t.Fatalf("job status = %q, want queued", updated.Status)
	}
}

func TestRabbitMQQueueHandleProcessingErrorSendsDLQ(t *testing.T) {
	store := newFakeJobStore()
	job, _, err := store.CreateOrGetActiveJob(context.Background(), "indexer", "book", "book-1", mustJSON(t, map[string]string{"bookId": "book-1"}))
	if err != nil {
		t.Fatalf("CreateOrGetActiveJob() error = %v", err)
	}
	job.Attempts = 3
	store.jobs[job.ID] = job

	queue, err := NewRabbitMQJobQueue(RabbitMQQueueConfig{
		URL:          "amqp://guest:guest@localhost:5672/",
		Exchange:     "onebook.jobs",
		QueueName:    "onebook.indexer.jobs",
		ConsumerName: "onebook-indexer-service",
		JobType:      "indexer",
		ResourceType: "book",
		MaxRetries:   3,
		RetryDelay:   0,
		Store:        store,
	})
	if err != nil {
		t.Fatalf("NewRabbitMQJobQueue() error = %v", err)
	}
	disposition, err := queue.handleProcessingError(context.Background(), job, context.Canceled)
	if err != nil {
		t.Fatalf("handleProcessingError() error = %v", err)
	}
	if disposition != failureDeadLetter {
		t.Fatalf("disposition = %v, want %v", disposition, failureDeadLetter)
	}
	if updated := store.jobs[job.ID]; updated.Status != StatusFailed {
		t.Fatalf("job status = %q, want failed", updated.Status)
	}
}

type fakeJobStore struct {
	jobs        map[string]JobStatus
	activeByKey map[string]string
}

func newFakeJobStore() *fakeJobStore {
	return &fakeJobStore{
		jobs:        map[string]JobStatus{},
		activeByKey: map[string]string{},
	}
}

func (s *fakeJobStore) CreateOrGetActiveJob(_ context.Context, jobType, resourceType, resourceID string, _ json.RawMessage) (JobStatus, bool, error) {
	key := jobType + ":" + resourceType + ":" + resourceID
	if id, ok := s.activeByKey[key]; ok {
		return s.jobs[id], false, nil
	}
	now := time.Now().UTC()
	job := JobStatus{
		ID:        resourceID + "-" + jobType,
		BookID:    resourceID,
		Status:    StatusQueued,
		Attempts:  0,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.jobs[job.ID] = job
	s.activeByKey[key] = job.ID
	return job, true, nil
}

func (s *fakeJobStore) GetJob(_ context.Context, jobID string) (JobStatus, bool, error) {
	job, ok := s.jobs[jobID]
	return job, ok, nil
}

func (s *fakeJobStore) MarkProcessing(_ context.Context, jobID string) (JobStatus, error) {
	job := s.jobs[jobID]
	if job.Status == StatusDone || job.Status == StatusFailed {
		return JobStatus{}, ErrJobTerminal
	}
	job.Status = StatusProcessing
	job.Attempts++
	job.UpdatedAt = time.Now().UTC()
	s.jobs[jobID] = job
	return job, nil
}

func (s *fakeJobStore) MarkQueued(_ context.Context, jobID, errMsg string) error {
	job := s.jobs[jobID]
	job.Status = StatusQueued
	job.ErrorMessage = errMsg
	job.UpdatedAt = time.Now().UTC()
	s.jobs[jobID] = job
	return nil
}

func (s *fakeJobStore) MarkDone(_ context.Context, jobID string) error {
	job := s.jobs[jobID]
	job.Status = StatusDone
	job.ErrorMessage = ""
	job.UpdatedAt = time.Now().UTC()
	s.jobs[jobID] = job
	return nil
}

func (s *fakeJobStore) MarkFailed(_ context.Context, jobID, errMsg string) error {
	job := s.jobs[jobID]
	job.Status = StatusFailed
	job.ErrorMessage = errMsg
	job.UpdatedAt = time.Now().UTC()
	s.jobs[jobID] = job
	return nil
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}
