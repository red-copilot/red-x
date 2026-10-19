package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

var QueueNames = []string{
	"q.discovery.network",
	"q.discovery.dns",
	"q.web.fetch",
	"q.web.scan",
	"q.poc.http",
	"q.poc.service",
	"q.monitor",
	"q.github",
	"q.maintenance",
	"q.icp",
}

type Envelope struct {
	ID        string          `json:"id"`
	Queue     string          `json:"queue"`
	Attempt   int             `json:"attempt"`
	TraceID   string          `json:"traceId,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
	Payload   json.RawMessage `json:"payload"`
}

type Delivery struct {
	Queue    string
	Envelope Envelope
}

type Handler func(context.Context, Delivery) error

type Queue interface {
	Publish(ctx context.Context, queue string, payload []byte) error
	Consume(ctx context.Context, queues []string, prefetch int, handler Handler) error
	Health(ctx context.Context) error
	Close() error
}

func NewEnvelope(queue string, payload []byte) (Envelope, []byte, error) {
	env := Envelope{
		ID: uuid.NewString(), Queue: queue, CreatedAt: time.Now().UTC(), Payload: payload,
	}
	b, err := json.Marshal(env)
	return env, b, err
}

type MemoryQueue struct {
	mu     sync.Mutex
	queues map[string]chan Delivery
	closed chan struct{}
	once   sync.Once
	log    *slog.Logger
}

func NewMemory(log *slog.Logger) *MemoryQueue {
	q := &MemoryQueue{queues: map[string]chan Delivery{}, closed: make(chan struct{}), log: log}
	for _, name := range QueueNames {
		q.queues[name] = make(chan Delivery, 1024)
	}
	return q
}

func (q *MemoryQueue) Publish(ctx context.Context, queue string, payload []byte) error {
	select {
	case <-q.closed:
		return errors.New("queue closed")
	default:
	}
	q.mu.Lock()
	ch, ok := q.queues[queue]
	q.mu.Unlock()
	if !ok {
		return fmt.Errorf("unknown queue %s", queue)
	}
	env, body, err := NewEnvelope(queue, payload)
	if err != nil {
		return err
	}
	select {
	case ch <- Delivery{Queue: queue, Envelope: env}:
		_ = body
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-q.closed:
		return errors.New("queue closed")
	}
}

func (q *MemoryQueue) Consume(ctx context.Context, queues []string, _ int, handler Handler) error {
	var wg sync.WaitGroup
	for _, name := range queues {
		q.mu.Lock()
		ch, ok := q.queues[name]
		q.mu.Unlock()
		if !ok {
			return fmt.Errorf("unknown queue %s", name)
		}
		wg.Add(1)
		go func(queue string, ch chan Delivery) {
			defer wg.Done()
			for {
				select {
				case d := <-ch:
					err := handler(ctx, d)
					if err != nil {
						q.log.Warn("memory job failed", "queue", queue, "attempt", d.Envelope.Attempt, "error", err)
						d.Envelope.Attempt++
						if d.Envelope.Attempt <= 3 {
							time.AfterFunc(time.Duration(d.Envelope.Attempt*d.Envelope.Attempt)*time.Second, func() {
								select {
								case ch <- d:
								case <-q.closed:
								}
							})
						}
					}
				case <-ctx.Done():
					return
				case <-q.closed:
					return
				}
			}
		}(name, ch)
	}
	wg.Wait()
	return ctx.Err()
}

func (q *MemoryQueue) Health(context.Context) error {
	select {
	case <-q.closed:
		return errors.New("queue closed")
	default:
		return nil
	}
}

func (q *MemoryQueue) Close() error {
	q.once.Do(func() { close(q.closed) })
	return nil
}

type RabbitQueue struct {
	url       string
	log       *slog.Logger
	mu        sync.Mutex
	pubConn   *amqp.Connection
	pubCh     *amqp.Channel
	confirmCh chan amqp.Confirmation
	closed    chan struct{}
	once      sync.Once
}

func NewRabbit(ctx context.Context, url string, log *slog.Logger) (*RabbitQueue, error) {
	q := &RabbitQueue{url: url, log: log, closed: make(chan struct{})}
	if err := q.connectPublisher(ctx); err != nil {
		return nil, err
	}
	return q, nil
}

func (q *RabbitQueue) connectPublisher(ctx context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.pubCh != nil && !q.pubCh.IsClosed() {
		return nil
	}
	conn, err := amqp.DialConfig(q.url, amqp.Config{Heartbeat: 15 * time.Second})
	if err != nil {
		return fmt.Errorf("rabbit dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return err
	}
	if err := declareTopology(ch); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return err
	}
	q.pubConn, q.pubCh = conn, ch
	q.confirmCh = ch.NotifyPublish(make(chan amqp.Confirmation, 128))
	return nil
}

func declareTopology(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare("arl.jobs", "direct", true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.ExchangeDeclare("arl.events", "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.ExchangeDeclare("arl.retry", "direct", true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.ExchangeDeclare("arl.dlx", "direct", true, false, false, false, nil); err != nil {
		return err
	}
	for _, name := range QueueNames {
		mainArgs := amqp.Table{"x-queue-type": "quorum"}
		if _, err := ch.QueueDeclare(name, true, false, false, false, mainArgs); err != nil {
			return err
		}
		if err := ch.QueueBind(name, name, "arl.jobs", false, nil); err != nil {
			return err
		}
		retry := name + ".retry"
		retryArgs := amqp.Table{
			"x-queue-type":              "quorum",
			"x-message-ttl":             int32(10000),
			"x-dead-letter-exchange":    "arl.jobs",
			"x-dead-letter-routing-key": name,
		}
		if _, err := ch.QueueDeclare(retry, true, false, false, false, retryArgs); err != nil {
			return err
		}
		if err := ch.QueueBind(retry, retry, "arl.jobs", false, nil); err != nil {
			return err
		}
		dlq := name + ".dlq"
		if _, err := ch.QueueDeclare(dlq, true, false, false, false, amqp.Table{"x-queue-type": "quorum"}); err != nil {
			return err
		}
		if err := ch.QueueBind(dlq, dlq, "arl.jobs", false, nil); err != nil {
			return err
		}
	}
	return nil
}

func (q *RabbitQueue) Publish(ctx context.Context, queue string, payload []byte) error {
	env, body, err := NewEnvelope(queue, payload)
	if err != nil {
		return err
	}
	_ = env
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.pubCh == nil || q.pubCh.IsClosed() {
		return errors.New("rabbit publisher is closed")
	}
	confirmation, err := q.pubCh.PublishWithDeferredConfirmWithContext(ctx, "arl.jobs", queue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now().UTC(),
		Body:         body,
	})
	if err != nil {
		return err
	}
	acked, err := confirmation.WaitContext(ctx)
	if err != nil {
		return err
	}
	if !acked {
		return errors.New("rabbit publish was not confirmed")
	}
	return nil
}

func (q *RabbitQueue) Consume(ctx context.Context, queues []string, prefetch int, handler Handler) error {
	if prefetch <= 0 {
		prefetch = 1
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-q.closed:
			return nil
		default:
		}
		if err := q.consumeOnce(ctx, queues, prefetch, handler); err != nil {
			q.log.Warn("rabbit consumer disconnected", "error", err)
			select {
			case <-time.After(3 * time.Second):
			case <-ctx.Done():
				return ctx.Err()
			case <-q.closed:
				return nil
			}
		}
	}
}

func (q *RabbitQueue) consumeOnce(ctx context.Context, queues []string, prefetch int, handler Handler) error {
	conn, err := amqp.DialConfig(q.url, amqp.Config{Heartbeat: 15 * time.Second})
	if err != nil {
		return err
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	if err := ch.Qos(prefetch, 0, false); err != nil {
		return err
	}
	for _, name := range queues {
		deliveries, err := ch.Consume(name, "arl-worker-"+uuid.NewString(), false, false, false, false, nil)
		if err != nil {
			return err
		}
		go func(queue string, deliveries <-chan amqp.Delivery) {
			for {
				select {
				case d, ok := <-deliveries:
					if !ok {
						return
					}
					var env Envelope
					if err := json.Unmarshal(d.Body, &env); err != nil {
						q.log.Error("invalid job envelope", "queue", queue, "error", err)
						_ = d.Nack(false, false)
						continue
					}
					err := handler(ctx, Delivery{Queue: queue, Envelope: env})
					if err == nil {
						_ = d.Ack(false)
						continue
					}
					env.Attempt++
					next, _ := json.Marshal(env)
					if env.Attempt <= 3 {
						if pubErr := ch.Publish("arl.jobs", queue+".retry", false, false, amqp.Publishing{
							ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: next,
						}); pubErr != nil {
							_ = d.Nack(false, true)
							continue
						}
					} else {
						_ = ch.Publish("arl.jobs", queue+".dlq", false, false, amqp.Publishing{
							ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: next,
						})
					}
					_ = d.Ack(false)
				case <-ctx.Done():
					return
				}
			}
		}(name, deliveries)
	}
	<-ctx.Done()
	return ctx.Err()
}

func (q *RabbitQueue) Health(context.Context) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.pubConn == nil || q.pubConn.IsClosed() {
		return errors.New("rabbit connection closed")
	}
	return nil
}

func (q *RabbitQueue) Close() error {
	q.once.Do(func() {
		close(q.closed)
		q.mu.Lock()
		defer q.mu.Unlock()
		if q.pubCh != nil {
			_ = q.pubCh.Close()
		}
		if q.pubConn != nil {
			_ = q.pubConn.Close()
		}
	})
	return nil
}
