package queue

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"onebookai/internal/util"
)

const (
	StatusQueued     = "queued"
	StatusProcessing = "processing"
	StatusDone       = "done"
	StatusFailed     = "failed"
)

type JobStatus struct {
	ID           string    `json:"id"`
	BookID       string    `json:"bookId"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"errorMessage,omitempty"`
	Attempts     int       `json:"attempts"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type RedisJobQueue struct {
	client       *redis.Client
	stream       string
	group        string
	consumerBase string
	jobTTL       time.Duration
	maxRetries   int
	block        time.Duration
	claimIdle    time.Duration
	retryDelay   time.Duration
	maxLen       int64
	readCount    int64
	claimCount   int64
	once         sync.Once
}

type RedisQueueConfig struct {
	Addr       string
	Password   string
	Stream     string
	Group      string
	Consumer   string
	JobTTL     time.Duration
	MaxRetries int
	Block      time.Duration
	ClaimIdle  time.Duration
	RetryDelay time.Duration
	MaxLen     int64
	ReadCount  int64
	ClaimCount int64
}

func NewRedisJobQueue(cfg RedisQueueConfig) (*RedisJobQueue, error) {
	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		return nil, errors.New("redis addr required")
	}
	stream := strings.TrimSpace(cfg.Stream)
	if stream == "" {
		return nil, errors.New("queue stream required")
	}
	group := strings.TrimSpace(cfg.Group)
	if group == "" {
		group = "default"
	}
	consumer := strings.TrimSpace(cfg.Consumer)
	if consumer == "" {
		consumer = util.NewID()
	}
	jobTTL := cfg.JobTTL
	if jobTTL <= 0 {
		jobTTL = 24 * time.Hour
	}
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	block := cfg.Block
	if block <= 0 {
		block = 5 * time.Second
	}
	claimIdle := cfg.ClaimIdle
	if claimIdle <= 0 {
		claimIdle = 30 * time.Second
	}
	retryDelay := cfg.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 2 * time.Second
	}
	maxLen := cfg.MaxLen
	if maxLen <= 0 {
		maxLen = 10000
	}
	readCount := cfg.ReadCount
	if readCount <= 0 {
		readCount = 10
	}
	claimCount := cfg.ClaimCount
	if claimCount <= 0 {
		claimCount = 10
	}

	return &RedisJobQueue{
		client:       redis.NewClient(&redis.Options{Addr: addr, Password: cfg.Password}),
		stream:       stream,
		group:        group,
		consumerBase: consumer,
		jobTTL:       jobTTL,
		maxRetries:   maxRetries,
		block:        block,
		claimIdle:    claimIdle,
		retryDelay:   retryDelay,
		maxLen:       maxLen,
		readCount:    readCount,
		claimCount:   claimCount,
	}, nil
}

func (q *RedisJobQueue) Enqueue(ctx context.Context, bookID string) (JobStatus, error) {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return JobStatus{}, errors.New("bookId required")
	}
	job := JobStatus{
		ID:        util.NewID(),
		BookID:    bookID,
		Status:    StatusQueued,
		Attempts:  0,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := q.writeStatus(ctx, job); err != nil {
		return JobStatus{}, err
	}
	if err := q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: q.stream,
		MaxLen: q.maxLen,
		Approx: true,
		Values: map[string]any{
			"job_id":  job.ID,
			"book_id": job.BookID,
		},
	}).Err(); err != nil {
		return JobStatus{}, err
	}
	return job, nil
}

func (q *RedisJobQueue) GetJob(ctx context.Context, jobID string) (JobStatus, bool, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return JobStatus{}, false, nil
	}
	key := q.jobKey(jobID)
	data, err := q.client.HGetAll(ctx, key).Result()
	if err != nil {
		return JobStatus{}, false, err
	}
	if len(data) == 0 {
		return JobStatus{}, false, nil
	}
	job, err := decodeJobStatus(jobID, data)
	if err != nil {
		return JobStatus{}, false, err
	}
	return job, true, nil
}

func (q *RedisJobQueue) Start(ctx context.Context, concurrency int, handler func(context.Context, JobStatus) error) {
	if concurrency <= 0 {
		concurrency = 1
	}
	q.ensureGroup(ctx)
	for i := 0; i < concurrency; i++ {
		consumer := fmt.Sprintf("%s-%d", q.consumerBase, i)
		go q.consumeLoop(ctx, consumer, handler)
	}
}

func (q *RedisJobQueue) ensureGroup(ctx context.Context) {
	q.once.Do(func() {
		err := q.client.XGroupCreateMkStream(ctx, q.stream, q.group, "$").Err()
		if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
			// best-effort; errors will surface on consume
		}
	})
}

func (q *RedisJobQueue) consumeLoop(ctx context.Context, consumer string, handler func(context.Context, JobStatus) error) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if msgs, err := q.claimPending(ctx, consumer); err == nil {
			for _, msg := range msgs {
				q.handleMessage(ctx, consumer, msg, handler)
			}
		}

		streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    q.group,
			Consumer: consumer,
			Streams:  []string{q.stream, ">"},
			Count:    q.readCount,
			Block:    q.block,
		}).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			}
			continue
		}
		for _, stream := range streams {
			for _, msg := range stream.Messages {
				q.handleMessage(ctx, consumer, msg, handler)
			}
		}
	}
}

func (q *RedisJobQueue) claimPending(ctx context.Context, consumer string) ([]redis.XMessage, error) {
	res, _, err := q.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   q.stream,
		Group:    q.group,
		Consumer: consumer,
		MinIdle:  q.claimIdle,
		Start:    "0-0",
		Count:    q.claimCount,
	}).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (q *RedisJobQueue) handleMessage(ctx context.Context, consumer string, msg redis.XMessage, handler func(context.Context, JobStatus) error) {
	jobID, _ := msg.Values["job_id"].(string)
	bookID, _ := msg.Values["book_id"].(string)
	if jobID == "" || bookID == "" {
		q.ackAndDel(ctx, msg.ID)
		return
	}
	job, err := q.markProcessing(ctx, jobID, bookID)
	if err != nil {
		q.ackAndDel(ctx, msg.ID)
		return
	}
	if err := handler(ctx, job); err == nil {
		_ = q.markDone(ctx, jobID)
		q.ackAndDel(ctx, msg.ID)
		return
	}
	if job.Attempts >= q.maxRetries {
		_ = q.markFailed(ctx, jobID, err.Error())
		q.ackAndDel(ctx, msg.ID)
		return
	}
	_ = q.markQueued(ctx, jobID, err.Error())
	if q.retryDelay > 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.After(q.retryDelay):
		}
	}
	_ = q.requeueAndAck(ctx, msg.ID, jobID, bookID)
}

func (q *RedisJobQueue) ackAndDel(ctx context.Context, msgID string) {
	_, _ = q.client.XAck(ctx, q.stream, q.group, msgID).Result()
	_, _ = q.client.XDel(ctx, q.stream, msgID).Result()
}

func (q *RedisJobQueue) requeueAndAck(ctx context.Context, msgID, jobID, bookID string) error {
	pipe := q.client.TxPipeline()
	pipe.XAdd(ctx, &redis.XAddArgs{
		Stream: q.stream,
		MaxLen: q.maxLen,
		Approx: true,
		Values: map[string]any{
			"job_id":  jobID,
			"book_id": bookID,
		},
	})
	pipe.XAck(ctx, q.stream, q.group, msgID)
	pipe.XDel(ctx, q.stream, msgID)
	_, err := pipe.Exec(ctx)
	return err
}

func (q *RedisJobQueue) markProcessing(ctx context.Context, jobID, bookID string) (JobStatus, error) {
	job, _, err := q.GetJob(ctx, jobID)
	if err != nil {
		return JobStatus{}, err
	}
	if job.ID == "" {
		job = JobStatus{ID: jobID}
	}
	if bookID != "" {
		job.BookID = bookID
	}
	job.Attempts++
	job.Status = StatusProcessing
	job.UpdatedAt = time.Now().UTC()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = job.UpdatedAt
	}
	if err := q.writeStatus(ctx, job); err != nil {
		return JobStatus{}, err
	}
	return job, nil
}

func (q *RedisJobQueue) markQueued(ctx context.Context, jobID, errMsg string) error {
	job, _, err := q.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	job.Status = StatusQueued
	job.ErrorMessage = errMsg
	job.UpdatedAt = time.Now().UTC()
	return q.writeStatus(ctx, job)
}

func (q *RedisJobQueue) markDone(ctx context.Context, jobID string) error {
	job, _, err := q.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	job.Status = StatusDone
	job.ErrorMessage = ""
	job.UpdatedAt = time.Now().UTC()
	return q.writeStatus(ctx, job)
}

func (q *RedisJobQueue) markFailed(ctx context.Context, jobID, errMsg string) error {
	job, _, err := q.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	job.Status = StatusFailed
	job.ErrorMessage = errMsg
	job.UpdatedAt = time.Now().UTC()
	return q.writeStatus(ctx, job)
}

func (q *RedisJobQueue) writeStatus(ctx context.Context, job JobStatus) error {
	key := q.jobKey(job.ID)
	payload := map[string]any{
		"id":        job.ID,
		"bookId":    job.BookID,
		"status":    job.Status,
		"error":     job.ErrorMessage,
		"attempts":  strconv.Itoa(job.Attempts),
		"createdAt": job.CreatedAt.Format(time.RFC3339Nano),
		"updatedAt": job.UpdatedAt.Format(time.RFC3339Nano),
	}
	if err := q.client.HSet(ctx, key, payload).Err(); err != nil {
		return err
	}
	_ = q.client.Expire(ctx, key, q.jobTTL).Err()
	return nil
}

func (q *RedisJobQueue) jobKey(jobID string) string {
	return fmt.Sprintf("job:%s:%s", q.stream, jobID)
}

func decodeJobStatus(jobID string, data map[string]string) (JobStatus, error) {
	job := JobStatus{ID: jobID}
	if v := data["bookId"]; v != "" {
		job.BookID = v
	}
	if v := data["status"]; v != "" {
		job.Status = v
	}
	if v := data["error"]; v != "" {
		job.ErrorMessage = v
	}
	if v := data["attempts"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			job.Attempts = n
		}
	}
	if v := data["createdAt"]; v != "" {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			job.CreatedAt = t
		}
	}
	if v := data["updatedAt"]; v != "" {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			job.UpdatedAt = t
		}
	}
	return job, nil
}
