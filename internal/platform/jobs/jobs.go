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
)

const (
	// maxRetries is the number of retries after the initial delivery.
	maxRetries = 3
	// retryDelay is the delay before a failed delivery is published again.
	retryDelay = 10 * time.Second
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

func knownQueue(queue string) bool {
	for _, name := range QueueNames {
		if name == queue {
			return true
		}
	}
	return false
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
	env, _, err := NewEnvelope(queue, payload)
	if err != nil {
		return err
	}
	select {
	case ch <- Delivery{Queue: queue, Envelope: env}:
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
						if d.Envelope.Attempt <= maxRetries {
							time.AfterFunc(retryDelay, func() {
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
