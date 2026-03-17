package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

// JobQueue defines queue operations used by async services.
type JobQueue interface {
	Enqueue(ctx context.Context, resourceID string) (JobStatus, error)
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

type kafkaWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type kafkaReader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

// KafkaQueueConfig configures the Kafka-backed queue.
type KafkaQueueConfig struct {
	Brokers      []string
	ClientID     string
	Topic        string
	DLQTopic     string
	Group        string
	JobType      string
	ResourceType string
	MaxRetries   int
	RetryDelay   time.Duration
	Store        jobStore
}

// KafkaJobQueue implements JobQueue using Kafka + Postgres-backed job state.
type KafkaJobQueue struct {
	brokers      []string
	clientID     string
	topic        string
	dlqTopic     string
	group        string
	jobType      string
	resourceType string
	maxRetries   int
	retryDelay   time.Duration
	store        jobStore
	writer       kafkaWriter
	dialer       *kafka.Dialer
	readerFn     func() kafkaReader
	publishFn    func(context.Context, string, JobEnvelope) error
}

func NewKafkaJobQueue(cfg KafkaQueueConfig) (*KafkaJobQueue, error) {
	brokers := compactStrings(cfg.Brokers)
	if len(brokers) == 0 {
		return nil, errors.New("kafka brokers required")
	}
	if strings.TrimSpace(cfg.Topic) == "" {
		return nil, errors.New("kafka topic required")
	}
	if strings.TrimSpace(cfg.Group) == "" {
		return nil, errors.New("kafka group required")
	}
	if strings.TrimSpace(cfg.JobType) == "" {
		return nil, errors.New("job type required")
	}
	if strings.TrimSpace(cfg.ResourceType) == "" {
		return nil, errors.New("resource type required")
	}
	if cfg.Store == nil {
		return nil, errors.New("job store required")
	}
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	retryDelay := cfg.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 2 * time.Second
	}
	clientID := strings.TrimSpace(cfg.ClientID)
	if clientID == "" {
		clientID = "onebook-app"
	}
	dlqTopic := strings.TrimSpace(cfg.DLQTopic)
	if dlqTopic == "" {
		dlqTopic = defaultDLQTopic(cfg.Topic)
	}

	dialer := &kafka.Dialer{
		ClientID:      clientID,
		Timeout:       10 * time.Second,
		DualStack:     true,
		KeepAlive:     30 * time.Second,
		Resolver:      nil,
		FallbackDelay: -1,
	}
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        false,
		Transport: &kafka.Transport{
			ClientID: clientID,
		},
	}

	q := &KafkaJobQueue{
		brokers:      brokers,
		clientID:     clientID,
		topic:        cfg.Topic,
		dlqTopic:     dlqTopic,
		group:        cfg.Group,
		jobType:      cfg.JobType,
		resourceType: cfg.ResourceType,
		maxRetries:   maxRetries,
		retryDelay:   retryDelay,
		store:        cfg.Store,
		writer:       writer,
		dialer:       dialer,
	}
	q.readerFn = func() kafkaReader {
		return kafka.NewReader(kafka.ReaderConfig{
			Brokers:               q.brokers,
			GroupID:               q.group,
			Topic:                 q.topic,
			MinBytes:              1,
			MaxBytes:              10e6,
			CommitInterval:        0,
			HeartbeatInterval:     3 * time.Second,
			SessionTimeout:        30 * time.Second,
			RebalanceTimeout:      30 * time.Second,
			ReadLagInterval:       -1,
			WatchPartitionChanges: true,
			ReadBackoffMin:        100 * time.Millisecond,
			ReadBackoffMax:        time.Second,
			MaxAttempts:           3,
			StartOffset:           kafka.FirstOffset,
		})
	}
	q.publishFn = q.publish
	return q, nil
}

func (q *KafkaJobQueue) Enqueue(ctx context.Context, resourceID string) (JobStatus, error) {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return JobStatus{}, errors.New("resourceId required")
	}
	payload, err := defaultPayload(q.resourceType, resourceID)
	if err != nil {
		return JobStatus{}, err
	}
	job, created, err := q.store.CreateOrGetActiveJob(ctx, q.jobType, q.resourceType, resourceID, payload)
	if err != nil {
		return JobStatus{}, err
	}
	if !created {
		return job, nil
	}
	env := JobEnvelope{
		JobID:        job.ID,
		JobType:      q.jobType,
		ResourceType: q.resourceType,
		ResourceID:   resourceID,
		Attempt:      job.Attempts,
		RequestedAt:  job.CreatedAt,
		Payload:      payload,
	}
	if err := q.publishFn(ctx, q.topic, env); err != nil {
		_ = q.store.MarkFailed(ctx, job.ID, err.Error())
		return JobStatus{}, err
	}
	return job, nil
}

func (q *KafkaJobQueue) GetJob(ctx context.Context, jobID string) (JobStatus, bool, error) {
	return q.store.GetJob(ctx, jobID)
}

func (q *KafkaJobQueue) Start(ctx context.Context, concurrency int, handler func(context.Context, JobStatus) error) {
	if concurrency <= 0 {
		concurrency = 1
	}
	for i := 0; i < concurrency; i++ {
		go func() {
			reader := q.readerFn()
			defer reader.Close()
			q.consumeLoop(ctx, reader, handler)
		}()
	}
}

func (q *KafkaJobQueue) Ready(ctx context.Context) error {
	if len(q.brokers) == 0 {
		return errors.New("kafka brokers not configured")
	}
	conn, err := q.dialer.DialContext(ctx, "tcp", q.brokers[0])
	if err != nil {
		return err
	}
	return conn.Close()
}

func (q *KafkaJobQueue) consumeLoop(ctx context.Context, reader kafkaReader, handler func(context.Context, JobStatus) error) {
	for {
		if ctx.Err() != nil {
			return
		}
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if err := q.handleMessage(ctx, msg, handler); err != nil {
			continue
		}
		if err := reader.CommitMessages(ctx, msg); err != nil && ctx.Err() == nil {
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func (q *KafkaJobQueue) handleMessage(ctx context.Context, msg kafka.Message, handler func(context.Context, JobStatus) error) error {
	env, err := DecodeJobEnvelope(msg.Value)
	if err != nil {
		if badErr := q.publishMalformed(ctx, msg, err); badErr != nil {
			return badErr
		}
		return nil
	}
	job, err := q.store.MarkProcessing(ctx, env.JobID)
	if err != nil {
		if errors.Is(err, ErrJobTerminal) {
			return nil
		}
		return err
	}
	if handlerErr := handler(ctx, job); handlerErr != nil {
		return q.handleProcessingError(ctx, job, env, handlerErr)
	}
	return q.store.MarkDone(ctx, job.ID)
}

func (q *KafkaJobQueue) handleProcessingError(ctx context.Context, job JobStatus, env JobEnvelope, handlerErr error) error {
	if job.Attempts >= q.maxRetries {
		if err := q.store.MarkFailed(ctx, job.ID, handlerErr.Error()); err != nil {
			return err
		}
		env.Attempt = job.Attempts
		return q.publishFn(ctx, q.dlqTopic, env)
	}
	if err := q.store.MarkQueued(ctx, job.ID, handlerErr.Error()); err != nil {
		return err
	}
	if q.retryDelay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(q.retryDelay):
		}
	}
	env.Attempt = job.Attempts
	env.RequestedAt = time.Now().UTC()
	return q.publishFn(ctx, q.topic, env)
}

func (q *KafkaJobQueue) publishMalformed(ctx context.Context, msg kafka.Message, decodeErr error) error {
	env := JobEnvelope{
		JobID:        fmt.Sprintf("malformed-%d", time.Now().UTC().UnixNano()),
		JobType:      "malformed",
		ResourceType: "message",
		ResourceID:   msg.Topic,
		RequestedAt:  time.Now().UTC(),
		Payload:      msg.Value,
	}
	return q.publishFn(ctx, q.dlqTopic, env)
}

func (q *KafkaJobQueue) publish(ctx context.Context, topic string, env JobEnvelope) error {
	payload, err := env.Encode()
	if err != nil {
		return err
	}
	key := env.ResourceID
	return q.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: payload,
		Time:  time.Now().UTC(),
		Headers: []kafka.Header{
			{Key: "job_id", Value: []byte(env.JobID)},
			{Key: "job_type", Value: []byte(env.JobType)},
			{Key: "resource_type", Value: []byte(env.ResourceType)},
			{Key: "resource_id", Value: []byte(env.ResourceID)},
		},
	})
}

func defaultDLQTopic(topic string) string {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return ""
	}
	if strings.HasSuffix(topic, ".jobs") {
		return strings.TrimSuffix(topic, ".jobs") + ".dlq"
	}
	return topic + ".dlq"
}

func defaultPayload(resourceType, resourceID string) (json.RawMessage, error) {
	body := map[string]string{
		resourceType + "Id": resourceID,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}
