package service_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/Brohammad/BugSathi/internal/platform/config"
	"github.com/Brohammad/BugSathi/internal/platform/pagination"
	"github.com/Brohammad/BugSathi/internal/projects/adapter/memory"
	"github.com/Brohammad/BugSathi/internal/projects/domain"
	"github.com/Brohammad/BugSathi/internal/projects/service"
	uploadmem "github.com/Brohammad/BugSathi/internal/uploads/adapter/memory"
	"github.com/google/uuid"
)

func TestProjectLifecycle(t *testing.T) {
	svc := service.New(memory.NewRepo(), uploadmem.NewStorage(), slog.Default(), config.ListConfig{})
	ctx := context.Background()
	owner := uuid.New()
	member := uuid.New()

	p, err := svc.Create(ctx, owner, " Acme ")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Acme" || p.Role != domain.RoleOwner {
		t.Fatalf("%+v", p)
	}

	got, err := svc.Get(ctx, owner, p.ID)
	if err != nil || got.ID != p.ID {
		t.Fatalf("get: %+v %v", got, err)
	}

	if _, err := svc.Get(ctx, member, p.ID); err != domain.ErrForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}

	m, err := svc.AddMember(ctx, owner, p.ID, member, "member")
	if err != nil {
		t.Fatal(err)
	}
	if m.Role != domain.RoleMember {
		t.Fatalf("%+v", m)
	}

	got, err = svc.Get(ctx, member, p.ID)
	if err != nil || got.Role != domain.RoleMember {
		t.Fatalf("member get: %+v %v", got, err)
	}

	if _, err := svc.Update(ctx, member, p.ID, "Nope"); err != domain.ErrForbidden {
		t.Fatalf("member update: %v", err)
	}

	updated, err := svc.Update(ctx, owner, p.ID, "Acme 2")
	if err != nil || updated.Name != "Acme 2" {
		t.Fatalf("update: %+v %v", updated, err)
	}

	list, err := svc.List(ctx, member, pagination.Page{Limit: 50})
	if err != nil || len(list.Items) != 1 {
		t.Fatalf("list: %+v %v", list, err)
	}

	if err := svc.Delete(ctx, member, p.ID); err != domain.ErrForbidden {
		t.Fatalf("member delete: %v", err)
	}
	if err := svc.Delete(ctx, owner, p.ID); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteRemovesProjectObjects(t *testing.T) {
	repo := memory.NewRepo()
	objs := uploadmem.NewStorage()
	svc := service.New(repo, objs, slog.Default(), config.ListConfig{})
	ctx := context.Background()
	owner := uuid.New()

	p, err := svc.Create(ctx, owner, "Cleanup")
	if err != nil {
		t.Fatal(err)
	}
	other := uuid.New()
	keepKey := "projects/" + other.String() + "/recordings/" + uuid.New().String() + "/source.webm"
	prefix := "projects/" + p.ID.String() + "/"
	source := prefix + "recordings/" + uuid.New().String() + "/source.webm"
	frame := prefix + "recordings/" + uuid.New().String() + "/frames/00000.jpg"
	thumb := prefix + "recordings/" + uuid.New().String() + "/thumb.jpg"
	objs.Put(source, "video/webm", []byte("src"))
	objs.Put(frame, "image/jpeg", []byte("frm"))
	objs.Put(thumb, "image/jpeg", []byte("thm"))
	objs.Put(keepKey, "video/webm", []byte("keep"))

	if err := svc.Delete(ctx, owner, p.ID); err != nil {
		t.Fatal(err)
	}
	if objs.Has(source) || objs.Has(frame) || objs.Has(thumb) {
		t.Fatalf("expected project objects deleted, still have: %v", objs.Keys())
	}
	if !objs.Has(keepKey) {
		t.Fatal("objects under another project must not be deleted")
	}
}

type failPrefix struct{}

func (failPrefix) DeletePrefix(context.Context, string) error {
	return errors.New("minio unavailable")
}

func TestDeleteSucceedsWhenObjectCleanupFails(t *testing.T) {
	repo := memory.NewRepo()
	svc := service.New(repo, failPrefix{}, slog.Default(), config.ListConfig{})
	ctx := context.Background()
	owner := uuid.New()
	p, err := svc.Create(ctx, owner, "Survive")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, owner, p.ID); err != nil {
		t.Fatalf("DB delete must succeed even if MinIO cleanup fails: %v", err)
	}
	if _, err := repo.GetByID(ctx, p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("project should be gone from DB, got %v", err)
	}
}
