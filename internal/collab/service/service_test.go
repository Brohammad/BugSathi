package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/Brohammad/BugSathi/internal/collab/adapter/hub"
	"github.com/Brohammad/BugSathi/internal/collab/adapter/memory"
	"github.com/Brohammad/BugSathi/internal/collab/domain"
	"github.com/Brohammad/BugSathi/internal/collab/service"
	"github.com/google/uuid"
)

func TestCommentAndPresenceFanout(t *testing.T) {
	repo := memory.NewRepo()
	h := hub.New()
	userID := uuid.New()
	projectID := uuid.New()
	reportID := uuid.New()
	svc := service.New(repo, memory.AccessOK{}, memory.ReportOK{}, memory.Authors{userID: "Dev"}, h)

	ch, unsub, err := svc.Subscribe(context.Background(), userID, projectID, reportID)
	if err != nil {
		t.Fatal(err)
	}
	defer unsub()

	// presence.updated on join
	select {
	case ev := <-ch:
		if ev.Type != domain.EventPresenceUpdated {
			t.Fatalf("want presence, got %s", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting presence")
	}

	c, err := svc.CreateComment(context.Background(), userID, projectID, reportID, " looks broken ")
	if err != nil {
		t.Fatal(err)
	}
	if c.Body != "looks broken" || c.AuthorName != "Dev" {
		t.Fatalf("%+v", c)
	}

	select {
	case ev := <-ch:
		if ev.Type != domain.EventCommentCreated {
			t.Fatalf("want comment, got %s", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting comment event")
	}

	list, err := svc.ListComments(context.Background(), userID, projectID, reportID)
	if err != nil || len(list) != 1 {
		t.Fatalf("%v %v", list, err)
	}
}

func TestForbidden(t *testing.T) {
	svc := service.New(memory.NewRepo(), memory.AccessDeny{}, memory.ReportOK{}, memory.Authors{}, hub.New())
	_, err := svc.CreateComment(context.Background(), uuid.New(), uuid.New(), uuid.New(), "x")
	if err != domain.ErrForbidden {
		t.Fatalf("got %v", err)
	}
}
