package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const defaultRabbitReconnectDelay = 2 * time.Second

// RabbitMQQueueConfig configures the RabbitMQ-backed queue.
type RabbitMQQueueConfig struct {
	URL            string
	Exchange       string
	QueueName      string
	DLQName        string
	ConsumerName   string
	JobType        string
	ResourceType   string
	MaxRetries     int
	RetryDelay     time.Duration
	ReconnectDelay time.Duration
	Store          jobStore
}

// RabbitMQJobQueue implements JobQueue using RabbitMQ + Postgres-backed job state.
type RabbitMQJobQueue struct {
	url            string
	exchange       string
	dlxExchange    string
	queueName      string
	dlqName        string
	consumerName   string
	jobType        string
	resourceType   string
	maxRetries     int
	retryDelay     time.Duration
	reconnectDelay time.Duration
	store          jobStore

	connMu    sync.Mutex
	conn      *amqp.Connection
	publishFn func(context.Context, string, JobEnvelope) error
}

type failureDisposition int

const (
	failureRequeue failureDisposition = iota
	failureDeadLetter
)

func NewRabbitMQJobQueue(cfg RabbitMQQueueConfig) (*RabbitMQJobQueue, error) {
	url := strings.TrimSpace(cfg.URL)
	if url == "" {
		return nil, errors.New("rabbitmq url required")
	}
	exchange := strings.TrimSpace(cfg.Exchange)
	if exchange == "" {
		return nil, errors.New("rabbitmq exchange required")
	}
	queueName := strings.TrimSpace(cfg.QueueName)
	if queueName == "" {
		return nil, errors.New("rabbitmq queue name required")
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
	reconnectDelay := cfg.ReconnectDelay
	if reconnectDelay <= 0 {
		reconnectDelay = defaultRabbitReconnectDelay
	}
	dlqName := strings.TrimSpace(cfg.DLQName)
	if dlqName == "" {
		dlqName = defaultDLQName(queueName)
	}
	consumerName := strings.TrimSpace(cfg.ConsumerName)
	if consumerName == "" {
		consumerName = queueName
	}
	q := &RabbitMQJobQueue{
		url:            url,
		exchange:       exchange,
		dlxExchange:    defaultDLXExchange(exchange),
		queueName:      queueName,
		dlqName:        dlqName,
		consumerName:   consumerName,
		jobType:        strings.TrimSpace(cfg.JobType),
		resourceType:   strings.TrimSpace(cfg.ResourceType),
		maxRetries:     maxRetries,
		retryDelay:     retryDelay,
		reconnectDelay: reconnectDelay,
		store:          cfg.Store,
	}
	q.publishFn = q.publish
	return q, nil
}

func (q *RabbitMQJobQueue) Enqueue(ctx context.Context, resourceID string) (JobStatus, error) {
	payload, err := defaultPayload(q.resourceType, resourceID)
	if err != nil {
		return JobStatus{}, err
	}
	return q.EnqueueWithPayload(ctx, resourceID, payload)
}

func (q *RabbitMQJobQueue) EnqueueWithPayload(ctx context.Context, resourceID string, payload json.RawMessage) (JobStatus, error) {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return JobStatus{}, errors.New("resourceId required")
	}
	payload = append([]byte(nil), payload...)
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
	if err := q.publishFn(ctx, q.queueName, env); err != nil {
		_ = q.store.MarkFailed(ctx, job.ID, err.Error())
		return JobStatus{}, err
	}
	return job, nil
}

func (q *RabbitMQJobQueue) GetJob(ctx context.Context, jobID string) (JobStatus, bool, error) {
	return q.store.GetJob(ctx, jobID)
}

func (q *RabbitMQJobQueue) Start(ctx context.Context, concurrency int, handler func(context.Context, JobStatus) error) {
	if concurrency <= 0 {
		concurrency = 1
	}
	for i := 0; i < concurrency; i++ {
		consumerTag := fmt.Sprintf("%s-%d", q.consumerName, i)
		go q.consumeLoop(ctx, consumerTag, handler)
	}
}

func (q *RabbitMQJobQueue) Ready(ctx context.Context) error {
	_, err := q.ensureTopology(ctx)
	return err
}

func (q *RabbitMQJobQueue) consumeLoop(ctx context.Context, consumerTag string, handler func(context.Context, JobStatus) error) {
	for ctx.Err() == nil {
		conn, err := q.ensureTopology(ctx)
		if err != nil {
			q.sleepReconnect(ctx)
			continue
		}
		ch, err := conn.Channel()
		if err != nil {
			q.resetConnection(conn)
			q.sleepReconnect(ctx)
			continue
		}
		if err := ch.Qos(1, 0, false); err != nil {
			_ = ch.Close()
			q.sleepReconnect(ctx)
			continue
		}
		deliveries, err := ch.Consume(q.queueName, consumerTag, false, false, false, false, nil)
		if err != nil {
			_ = ch.Close()
			q.resetConnection(conn)
			q.sleepReconnect(ctx)
			continue
		}
		if !q.consumeDeliveries(ctx, ch, deliveries, handler) {
			return
		}
	}
}

func (q *RabbitMQJobQueue) consumeDeliveries(ctx context.Context, ch *amqp.Channel, deliveries <-chan amqp.Delivery, handler func(context.Context, JobStatus) error) bool {
	defer ch.Close()
	for {
		select {
		case <-ctx.Done():
			return false
		case msg, ok := <-deliveries:
			if !ok {
				return true
			}
			if err := q.handleDelivery(ctx, msg, handler); err != nil {
				continue
			}
		}
	}
}

func (q *RabbitMQJobQueue) handleDelivery(ctx context.Context, msg amqp.Delivery, handler func(context.Context, JobStatus) error) error {
	env, err := DecodeJobEnvelope(msg.Body)
	if err != nil {
		return msg.Nack(false, false)
	}
	job, err := q.store.MarkProcessing(ctx, env.JobID)
	if err != nil {
		if errors.Is(err, ErrJobTerminal) {
			return msg.Ack(false)
		}
		_ = msg.Nack(false, true)
		return err
	}
	if handlerErr := handler(ctx, job); handlerErr != nil {
		disposition, err := q.handleProcessingError(ctx, job, handlerErr)
		if err != nil {
			_ = msg.Nack(false, true)
			return err
		}
		switch disposition {
		case failureDeadLetter:
			return msg.Nack(false, false)
		default:
			if q.retryDelay > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(q.retryDelay):
				}
			}
			return msg.Nack(false, true)
		}
	}
	if err := q.store.MarkDone(ctx, job.ID); err != nil {
		_ = msg.Nack(false, true)
		return err
	}
	return msg.Ack(false)
}

func (q *RabbitMQJobQueue) handleProcessingError(ctx context.Context, job JobStatus, handlerErr error) (failureDisposition, error) {
	if job.Attempts >= q.maxRetries {
		if err := q.store.MarkFailed(ctx, job.ID, handlerErr.Error()); err != nil {
			return failureRequeue, err
		}
		return failureDeadLetter, nil
	}
	if err := q.store.MarkQueued(ctx, job.ID, handlerErr.Error()); err != nil {
		return failureRequeue, err
	}
	return failureRequeue, nil
}

func (q *RabbitMQJobQueue) publish(ctx context.Context, routingKey string, env JobEnvelope) error {
	conn, err := q.ensureTopology(ctx)
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		q.resetConnection(conn)
		return err
	}
	defer ch.Close()
	if err := ch.Confirm(false); err != nil {
		return err
	}
	confirmations := ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	payload, err := env.Encode()
	if err != nil {
		return err
	}
	msg := amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now().UTC(),
		Type:         env.JobType,
		MessageId:    env.JobID,
		Body:         payload,
		Headers: amqp.Table{
			"job_id":        env.JobID,
			"job_type":      env.JobType,
			"resource_type": env.ResourceType,
			"resource_id":   env.ResourceID,
		},
	}
	if err := ch.PublishWithContext(ctx, q.exchange, routingKey, false, false, msg); err != nil {
		q.resetConnection(conn)
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case confirmation, ok := <-confirmations:
		if !ok {
			return errors.New("rabbitmq publish confirmation channel closed")
		}
		if !confirmation.Ack {
			return errors.New("rabbitmq publish not acknowledged")
		}
		return nil
	}
}

func (q *RabbitMQJobQueue) ensureTopology(_ context.Context) (*amqp.Connection, error) {
	q.connMu.Lock()
	defer q.connMu.Unlock()

	if q.conn != nil && !q.conn.IsClosed() {
		return q.conn, nil
	}

	conn, err := amqp.Dial(q.url)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	defer ch.Close()

	if err := ch.ExchangeDeclare(q.exchange, "direct", true, false, false, false, nil); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := ch.ExchangeDeclare(q.dlxExchange, "direct", true, false, false, false, nil); err != nil {
		_ = conn.Close()
		return nil, err
	}
	mainArgs := amqp.Table{
		"x-dead-letter-exchange":    q.dlxExchange,
		"x-dead-letter-routing-key": q.dlqName,
	}
	if _, err := ch.QueueDeclare(q.queueName, true, false, false, false, mainArgs); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if _, err := ch.QueueDeclare(q.dlqName, true, false, false, false, nil); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := ch.QueueBind(q.queueName, q.queueName, q.exchange, false, nil); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := ch.QueueBind(q.dlqName, q.dlqName, q.dlxExchange, false, nil); err != nil {
		_ = conn.Close()
		return nil, err
	}

	q.conn = conn
	return q.conn, nil
}

func (q *RabbitMQJobQueue) resetConnection(conn *amqp.Connection) {
	q.connMu.Lock()
	defer q.connMu.Unlock()
	if conn != nil && q.conn == conn {
		_ = conn.Close()
		q.conn = nil
	}
}

func (q *RabbitMQJobQueue) sleepReconnect(ctx context.Context) {
	select {
	case <-ctx.Done():
	case <-time.After(q.reconnectDelay):
	}
}

func defaultDLQName(queueName string) string {
	queueName = strings.TrimSpace(queueName)
	if queueName == "" {
		return ""
	}
	if strings.HasSuffix(queueName, ".jobs") {
		return strings.TrimSuffix(queueName, ".jobs") + ".dlq"
	}
	return queueName + ".dlq"
}

func defaultDLXExchange(exchange string) string {
	exchange = strings.TrimSpace(exchange)
	if exchange == "" {
		return ""
	}
	return exchange + ".dlx"
}
