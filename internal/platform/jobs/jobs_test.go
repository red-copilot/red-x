package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/Aabyss-Team/ARL/internal/platform/logx"
)

func TestMemoryQueue(t *testing.T) {
	q := NewMemory(logx.New("error", false))
	defer q.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	got := make(chan string, 1)
	go func() {
		_ = q.Consume(ctx, []string{"q.maintenance"}, 1, func(_ context.Context, d Delivery) error {
			got <- string(d.Envelope.Payload)
			return nil
		})
	}()
	if err := q.Publish(ctx, "q.maintenance", []byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
	select {
	case payload := <-got:
		if payload != `{"ok":true}` {
			t.Fatalf("unexpected payload: %s", payload)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for job")
	}
}
