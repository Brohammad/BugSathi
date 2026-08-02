package service_test

import (
	"context"
	"testing"

	"github.com/Brohammad/BugSathi/internal/projects/adapter/memory"
	"github.com/Brohammad/BugSathi/internal/projects/domain"
	"github.com/Brohammad/BugSathi/internal/projects/service"
	"github.com/google/uuid"
)

func TestProjectLifecycle(t *testing.T) {
	svc := service.New(memory.NewRepo())
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

	list, err := svc.List(ctx, member)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %+v %v", list, err)
	}

	if err := svc.Delete(ctx, member, p.ID); err != domain.ErrForbidden {
		t.Fatalf("member delete: %v", err)
	}
	if err := svc.Delete(ctx, owner, p.ID); err != nil {
		t.Fatal(err)
	}
}
