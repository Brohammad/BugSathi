package hub_test

import (
	"testing"
	"time"

	"github.com/Brohammad/BugSathi/internal/collab/adapter/hub"
	"github.com/Brohammad/BugSathi/internal/collab/domain"
	"github.com/Brohammad/BugSathi/internal/collab/port"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/google/uuid"
)

func TestRedisHubCrossReplicaFanout(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb1 := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	rdb2 := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rdb1.Close()
		_ = rdb2.Close()
	})

	h1 := hub.NewRedis(rdb1)
	defer h1.Close()
	h2 := hub.NewRedis(rdb2)
	defer h2.Close()

	reportID := uuid.New()
	userID := uuid.New()
	ch, unsub := h1.Subscribe(reportID, userID, "Dev")
	defer unsub()

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for presence")
	}

	h2.Publish(reportID, port.StreamEvent{Type: domain.EventCommentCreated, Data: map[string]string{"body": "hi"}})

	select {
	case ev := <-ch:
		if ev.Type != domain.EventCommentCreated {
			t.Fatalf("got %s", ev.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for cross-replica event")
	}
}
