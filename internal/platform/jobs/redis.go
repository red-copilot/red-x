package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	redisConsumerGroup = "arl-workers"
	redisReclaimIdle   = 2 * time.Minute
	redisLeaseRefresh  = 30 * time.Second
	redisReadBlock     = 2 * time.Second
	redisKeyPrefix     = "arl:jobs:"
)

const promoteRetryScript = `
local items = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1], 'LIMIT', 0, ARGV[2])
for _, item in ipairs(items) do
  redis.call('XADD', KEYS[2], '*', 'body', item)
  redis.call('ZREM', KEYS[1], item)
end
return #items
`

type RedisQueue struct {
	client      *redis.Client
	log         *slog.Logger
	consumer    string
	closed      chan struct{}
	once        sync.Once
	retryDelay  time.Duration
	reclaimIdle time.Duration
}

func NewRedis(ctx context.Context, url string, log *slog.Logger) (*RedisQueue, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	if log == nil {
		log = slog.Default()
	}
	q := &RedisQueue{
		client:      redis.NewClient(opts),
		log:         log,
		consumer:    "arl-worker-" + uuid.NewString(),
		closed:      make(chan struct{}),
		retryDelay:  retryDelay,
		reclaimIdle: redisReclaimIdle,
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := q.client.Ping(pingCtx).Err(); err != nil {
		_ = q.client.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	if err := q.ensureGroups(pingCtx, QueueNames); err != nil {
		_ = q.client.Close()
		return nil, err
	}
	return q, nil
}

func streamKey(queue string) string {
	return redisKeyPrefix + queue
}

func retryKey(queue string) string {
	return streamKey(queue) + ":retry"
}

func dlqKey(queue string) string {
	return streamKey(queue) + ":dlq"
}

func (q *RedisQueue) ensureGroups(ctx context.Context, queues []string) error {
	for _, queue := range queues {
		err := q.client.XGroupCreateMkStream(ctx, streamKey(queue), redisConsumerGroup, "0").Err()
		if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
			return fmt.Errorf("create redis consumer group for %s: %w", queue, err)
		}
	}
	return nil
}

func (q *RedisQueue) Publish(ctx context.Context, queue string, payload []byte) error {
	if !knownQueue(queue) {
		return fmt.Errorf("unknown queue %s", queue)
	}
	select {
	case <-q.closed:
		return errors.New("queue closed")
	default:
	}
	_, body, err := NewEnvelope(queue, payload)
	if err != nil {
		return err
	}
	return q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey(queue),
		Values: []any{"body", string(body)},
	}).Err()
}

func (q *RedisQueue) Consume(ctx context.Context, queues []string, prefetch int, handler Handler) error {
	if prefetch <= 0 {
		prefetch = 1
	}
	for _, queue := range queues {
		if !knownQueue(queue) {
			return fmt.Errorf("unknown queue %s", queue)
		}
	}
	if err := q.ensureGroups(ctx, queues); err != nil {
		return err
	}

	consumeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go q.promoteRetries(consumeCtx, queues)

	var wg sync.WaitGroup
	for _, queue := range queues {
		for i := 0; i < prefetch; i++ {
			wg.Add(1)
			go func(queue string) {
				defer wg.Done()
				q.consumeLoop(consumeCtx, queue, handler)
			}(queue)
		}
	}
	wg.Wait()

	select {
	case <-q.closed:
		return nil
	default:
	}
	return ctx.Err()
}

func (q *RedisQueue) consumeLoop(ctx context.Context, queue string, handler Handler) {
	stream := streamKey(queue)
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.closed:
			return
		default:
		}

		claimed, err := q.claimPending(ctx, stream, queue, handler)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if strings.Contains(err.Error(), "NOGROUP") {
				if groupErr := q.ensureGroups(ctx, []string{queue}); groupErr != nil {
					q.log.Warn("recreate redis consumer group failed", "queue", queue, "error", groupErr)
				}
				continue
			}
			q.log.Warn("redis pending claim failed", "queue", queue, "error", err)
			q.sleep(ctx, time.Second)
			continue
		}
		if claimed {
			continue
		}

		if err := q.readNew(ctx, stream, queue, handler); err != nil {
			if ctx.Err() != nil {
				return
			}
			if errors.Is(err, redis.Nil) {
				continue
			}
			if strings.Contains(err.Error(), "NOGROUP") {
				if groupErr := q.ensureGroups(ctx, []string{queue}); groupErr != nil {
					q.log.Warn("recreate redis consumer group failed", "queue", queue, "error", groupErr)
				}
				continue
			}
			q.log.Warn("redis read failed", "queue", queue, "error", err)
			q.sleep(ctx, time.Second)
		}
	}
}

func (q *RedisQueue) claimPending(ctx context.Context, stream, queue string, handler Handler) (bool, error) {
	messages, _, err := q.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   stream,
		Group:    redisConsumerGroup,
		Consumer: q.consumer,
		MinIdle:  q.reclaimIdle,
		Start:    "0-0",
		Count:    1,
	}).Result()
	if err != nil {
		return false, err
	}
	if len(messages) == 0 {
		return false, nil
	}
	q.handleMessage(ctx, stream, queue, messages[0], handler)
	return true, nil
}

func (q *RedisQueue) readNew(ctx context.Context, stream, queue string, handler Handler) error {
	streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    redisConsumerGroup,
		Consumer: q.consumer,
		Streams:  []string{stream, ">"},
		Count:    1,
		Block:    redisReadBlock,
	}).Result()
	if err != nil {
		return err
	}
	for _, streamResult := range streams {
		for _, message := range streamResult.Messages {
			q.handleMessage(ctx, stream, queue, message, handler)
		}
	}
	return nil
}

func (q *RedisQueue) handleMessage(ctx context.Context, stream, queue string, message redis.XMessage, handler Handler) {
	body, ok := message.Values["body"].(string)
	if !ok {
		q.log.Error("redis job has no body", "queue", queue, "id", message.ID)
		body = fmt.Sprintf(`{"error":"missing body","messageId":%q}`, message.ID)
		if err := q.deadLetter(ctx, queue, body); err != nil {
			q.log.Warn("write redis dlq failed", "queue", queue, "id", message.ID, "error", err)
			return
		}
		q.finish(ctx, stream, message.ID)
		return
	}

	var env Envelope
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		q.log.Error("invalid job envelope", "queue", queue, "id", message.ID, "error", err)
		if err := q.deadLetter(ctx, queue, body); err != nil {
			q.log.Warn("write redis dlq failed", "queue", queue, "id", message.ID, "error", err)
			return
		}
		q.finish(ctx, stream, message.ID)
		return
	}
	if env.Queue == "" {
		env.Queue = queue
	}

	stopLease := q.keepLease(ctx, stream, message.ID)
	err := handler(ctx, Delivery{Queue: queue, Envelope: env})
	stopLease()
	if ctx.Err() != nil {
		// Shutdown or caller cancellation: leave the message pending so it can
		// be reclaimed by another worker after the visibility timeout.
		return
	}
	if err == nil {
		q.finish(ctx, stream, message.ID)
		return
	}

	env.Attempt++
	next, marshalErr := json.Marshal(env)
	if marshalErr != nil {
		q.log.Error("marshal retry envelope failed", "queue", queue, "error", marshalErr)
		return
	}
	if env.Attempt <= maxRetries {
		err = q.client.ZAdd(ctx, retryKey(queue), redis.Z{
			Score:  float64(time.Now().Add(q.retryDelay).UnixMilli()),
			Member: string(next),
		}).Err()
		if err != nil {
			q.log.Warn("schedule redis retry failed", "queue", queue, "attempt", env.Attempt, "error", err)
			return
		}
	} else {
		if err := q.deadLetter(ctx, queue, string(next)); err != nil {
			q.log.Warn("write redis dlq failed", "queue", queue, "attempt", env.Attempt, "error", err)
			return
		}
	}
	q.finish(ctx, stream, message.ID)
}

func (q *RedisQueue) keepLease(ctx context.Context, stream, id string) func() {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(redisLeaseRefresh)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-q.closed:
				return
			case <-ticker.C:
				refreshCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				err := q.client.XClaimJustID(refreshCtx, &redis.XClaimArgs{
					Stream:   stream,
					Group:    redisConsumerGroup,
					Consumer: q.consumer,
					MinIdle:  0,
					Messages: []string{id},
				}).Err()
				cancel()
				if err != nil && !errors.Is(err, redis.Nil) {
					q.log.Warn("refresh redis lease failed", "stream", stream, "id", id, "error", err)
				}
			}
		}
	}()
	return func() { close(done) }
}

func (q *RedisQueue) finish(ctx context.Context, stream, id string) {
	ackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := q.client.XAck(ackCtx, stream, redisConsumerGroup, id).Err(); err != nil {
		if ctx.Err() == nil {
			q.log.Warn("redis ack failed", "stream", stream, "id", id, "error", err)
		}
		return
	}
	if err := q.client.XDel(ackCtx, stream, id).Err(); err != nil && ctx.Err() == nil {
		q.log.Warn("redis stream delete failed", "stream", stream, "id", id, "error", err)
	}
}

func (q *RedisQueue) deadLetter(ctx context.Context, queue, body string) error {
	if body == "" {
		return errors.New("empty dlq body")
	}
	return q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: dlqKey(queue),
		Values: []any{"body", body},
	}).Err()
}

func (q *RedisQueue) promoteRetries(ctx context.Context, queues []string) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.closed:
			return
		case <-ticker.C:
			now := time.Now().UnixMilli()
			for _, queue := range queues {
				if err := q.client.Eval(ctx, promoteRetryScript, []string{retryKey(queue), streamKey(queue)}, now, 50).Err(); err != nil {
					if ctx.Err() == nil {
						q.log.Warn("promote redis retries failed", "queue", queue, "error", err)
					}
				}
			}
		}
	}
}

func (q *RedisQueue) sleep(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-q.closed:
	case <-timer.C:
	}
}

func (q *RedisQueue) Health(ctx context.Context) error {
	select {
	case <-q.closed:
		return errors.New("queue closed")
	default:
	}
	return q.client.Ping(ctx).Err()
}

func (q *RedisQueue) Close() error {
	var err error
	q.once.Do(func() {
		close(q.closed)
		err = q.client.Close()
	})
	return err
}
