package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/red-copilot/red-x/internal/platform/logx"
)

const testQueue = "q.maintenance"

func testRedisQueue(t *testing.T) *RedisQueue {
	t.Helper()
	url := os.Getenv("ARL_TEST_REDIS_URL")
	if url == "" {
		t.Skip("ARL_TEST_REDIS_URL is not set")
	}
	q, err := NewRedis(context.Background(), url, logx.New("error", false))
	if err != nil {
		t.Fatalf("connect redis: %v", err)
	}
	t.Cleanup(func() { _ = q.Close() })
	return q
}

func resetRedisQueue(t *testing.T, q *RedisQueue, queue string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := q.client.Del(ctx, streamKey(queue), retryKey(queue), dlqKey(queue)).Err(); err != nil {
		t.Fatalf("reset redis queue: %v", err)
	}
	if err := q.ensureGroups(ctx, []string{queue}); err != nil {
		t.Fatalf("ensure groups: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = q.client.Del(ctx, streamKey(queue), retryKey(queue), dlqKey(queue)).Err()
	})
}

func waitFor(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func startTestConsumer(t *testing.T, q *RedisQueue, ctx context.Context, handler Handler) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- q.Consume(ctx, []string{testQueue}, 1, handler)
	}()
	return done
}

func TestRedisPublishConsume(t *testing.T) {
	q := testRedisQueue(t)
	resetRedisQueue(t, q, testQueue)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	got := make(chan Delivery, 1)
	done := startTestConsumer(t, q, ctx, func(_ context.Context, d Delivery) error {
		got <- d
		return nil
	})
	if err := q.Publish(ctx, testQueue, []byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
	select {
	case d := <-got:
		if string(d.Envelope.Payload) != `{"ok":true}` {
			t.Fatalf("payload = %s", d.Envelope.Payload)
		}
		if d.Queue != testQueue {
			t.Fatalf("queue = %s", d.Queue)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for job")
	}
	waitFor(t, 3*time.Second, func() bool {
		n, err := q.client.XLen(context.Background(), streamKey(testQueue)).Result()
		return err == nil && n == 0
	})
	cancel()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("consume returned %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("consumer did not stop")
	}
}

func TestRedisRetryThenSuccess(t *testing.T) {
	q := testRedisQueue(t)
	resetRedisQueue(t, q, testQueue)
	q.retryDelay = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	attempts := 0
	success := make(chan int, 1)
	done := startTestConsumer(t, q, ctx, func(_ context.Context, _ Delivery) error {
		attempts++
		if attempts < 2 {
			return errors.New("transient")
		}
		success <- attempts
		return nil
	})
	if err := q.Publish(ctx, testQueue, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-success:
		if got != 2 {
			t.Fatalf("attempts = %d, want 2", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for retry")
	}
	waitFor(t, 3*time.Second, func() bool {
		n, err := q.client.ZCard(context.Background(), retryKey(testQueue)).Result()
		return err == nil && n == 0
	})
	cancel()
	<-done
}

func TestRedisDLQAfterMaxRetries(t *testing.T) {
	q := testRedisQueue(t)
	resetRedisQueue(t, q, testQueue)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	env, body, err := NewEnvelope(testQueue, []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	env.Attempt = maxRetries
	body, err = json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.client.XAdd(ctx, &redis.XAddArgs{Stream: streamKey(testQueue), Values: []any{"body", string(body)}}).Err(); err != nil {
		t.Fatal(err)
	}
	done := startTestConsumer(t, q, ctx, func(_ context.Context, _ Delivery) error {
		return errors.New("permanent")
	})
	waitFor(t, 3*time.Second, func() bool {
		n, err := q.client.XLen(context.Background(), dlqKey(testQueue)).Result()
		return err == nil && n == 1
	})
	cancel()
	<-done

	messages, err := q.client.XRange(context.Background(), dlqKey(testQueue), "-", "+").Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("dlq length = %d", len(messages))
	}
	var got Envelope
	if err := json.Unmarshal([]byte(messages[0].Values["body"].(string)), &got); err != nil {
		t.Fatal(err)
	}
	if got.Attempt != maxRetries+1 {
		t.Fatalf("dlq attempt = %d, want %d", got.Attempt, maxRetries+1)
	}
}

func TestRedisClaimsOrphanPending(t *testing.T) {
	q := testRedisQueue(t)
	resetRedisQueue(t, q, testQueue)
	q.reclaimIdle = 20 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, body, err := NewEnvelope(testQueue, []byte(`{"orphan":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := q.client.XAdd(ctx, &redis.XAddArgs{Stream: streamKey(testQueue), Values: []any{"body", string(body)}}).Err(); err != nil {
		t.Fatal(err)
	}
	streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    redisConsumerGroup,
		Consumer: "orphan",
		Streams:  []string{streamKey(testQueue), ">"},
		Count:    1,
		Block:    -1,
	}).Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 1 || len(streams[0].Messages) != 1 {
		t.Fatalf("unexpected read result: %+v", streams)
	}
	time.Sleep(30 * time.Millisecond)

	got := make(chan Delivery, 1)
	claimed, err := q.claimPending(ctx, streamKey(testQueue), testQueue, func(_ context.Context, d Delivery) error {
		got <- d
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !claimed {
		t.Fatal("expected orphaned message to be claimed")
	}
	select {
	case d := <-got:
		if string(d.Envelope.Payload) != `{"orphan":true}` {
			t.Fatalf("payload = %s", d.Envelope.Payload)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for claimed job")
	}
}

func TestRedisInvalidEnvelopeGoesToDLQ(t *testing.T) {
	q := testRedisQueue(t)
	resetRedisQueue(t, q, testQueue)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := q.client.XAdd(ctx, &redis.XAddArgs{Stream: streamKey(testQueue), Values: []any{"body", "{"}}).Err(); err != nil {
		t.Fatal(err)
	}
	called := make(chan struct{}, 1)
	done := startTestConsumer(t, q, ctx, func(_ context.Context, _ Delivery) error {
		called <- struct{}{}
		return nil
	})
	waitFor(t, 3*time.Second, func() bool {
		n, err := q.client.XLen(context.Background(), dlqKey(testQueue)).Result()
		return err == nil && n == 1
	})
	select {
	case <-called:
		t.Fatal("handler should not run for invalid envelope")
	case <-time.After(100 * time.Millisecond):
	}
	cancel()
	<-done
}

func TestRedisHealthAndUnknownQueue(t *testing.T) {
	q := testRedisQueue(t)
	if err := q.Health(context.Background()); err != nil {
		t.Fatalf("health: %v", err)
	}
	if err := q.Publish(context.Background(), "q.unknown", []byte(`{}`)); err == nil {
		t.Fatal("expected unknown queue error")
	}
	if err := q.Close(); err != nil {
		t.Fatal(err)
	}
	if err := q.Health(context.Background()); err == nil {
		t.Fatal("expected closed queue health error")
	}
}
