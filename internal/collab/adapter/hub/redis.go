package hub

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/Brohammad/BugSathi/internal/collab/port"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

const reportFanoutChannel = "bugsathi:report-events"

type redisEnvelope struct {
	ReportID string           `json:"report_id"`
	Event    port.StreamEvent `json:"event"`
}

// RedisHub fans out SSE events across API replicas via Redis pub/sub while keeping
// local subscriber channels in MemoryHub. Presence remains per-instance (ADR 0031).
type RedisHub struct {
	local  *MemoryHub
	rdb    *goredis.Client
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewRedis(rdb *goredis.Client) *RedisHub {
	local := New()
	ctx, cancel := context.WithCancel(context.Background())
	h := &RedisHub{local: local, rdb: rdb, cancel: cancel}
	ready := make(chan struct{})
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		h.forward(ctx, ready)
	}()
	<-ready
	return h
}

func (h *RedisHub) Close() {
	if h.cancel != nil {
		h.cancel()
	}
	h.wg.Wait()
}

func (h *RedisHub) Subscribe(reportID, userID uuid.UUID, name string) (<-chan port.StreamEvent, func()) {
	return h.local.Subscribe(reportID, userID, name)
}

func (h *RedisHub) Publish(reportID uuid.UUID, ev port.StreamEvent) {
	if h.rdb == nil {
		h.local.Publish(reportID, ev)
		return
	}
	payload, err := json.Marshal(redisEnvelope{ReportID: reportID.String(), Event: ev})
	if err != nil {
		return
	}
	_ = h.rdb.Publish(context.Background(), reportFanoutChannel, payload).Err()
}

func (h *RedisHub) Presence(reportID uuid.UUID) []port.PresenceUser {
	return h.local.Presence(reportID)
}

func (h *RedisHub) forward(ctx context.Context, ready chan<- struct{}) {
	sub := h.rdb.Subscribe(ctx, reportFanoutChannel)
	defer sub.Close()
	if _, err := sub.Receive(ctx); err != nil {
		close(ready)
		return
	}
	close(ready)
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var env redisEnvelope
			if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
				continue
			}
			reportID, err := uuid.Parse(env.ReportID)
			if err != nil {
				continue
			}
			h.local.Publish(reportID, env.Event)
		}
	}
}

var _ port.Hub = (*RedisHub)(nil)
