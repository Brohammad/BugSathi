package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/Brohammad/BugSathi/internal/auth/adapter/jwtmgr"
	"github.com/Brohammad/BugSathi/internal/auth/adapter/memory"
	"github.com/Brohammad/BugSathi/internal/auth/adapter/password"
	"github.com/Brohammad/BugSathi/internal/auth/domain"
	"github.com/Brohammad/BugSathi/internal/auth/service"
)

func newTestService(t *testing.T) *service.Service {
	t.Helper()
	tm, err := jwtmgr.New("0123456789abcdef0123456789abcdef", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return service.New(
		memory.NewUserRepo(),
		memory.NewRefreshRepo(),
		password.NewArgon2id(),
		tm,
		7*24*time.Hour,
	)
}

func TestRegisterLoginMeRefreshLogout(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	user, pair, err := svc.Register(ctx, "Dev@Example.com", "password123", "Dev")
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "dev@example.com" {
		t.Fatalf("email = %q", user.Email)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("expected tokens")
	}

	_, _, err = svc.Register(ctx, "dev@example.com", "password123", "Dev")
	if err != domain.ErrEmailTaken {
		t.Fatalf("expected email taken, got %v", err)
	}

	_, loginPair, err := svc.Login(ctx, "dev@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}

	uid, email, err := svc.ParseAccess(loginPair.AccessToken)
	if err != nil || uid != user.ID || email != user.Email {
		t.Fatalf("parse access: uid=%v email=%q err=%v", uid, email, err)
	}

	me, err := svc.Me(ctx, uid)
	if err != nil || me.ID != user.ID {
		t.Fatalf("me: %#v err=%v", me, err)
	}

	rotated, err := svc.Refresh(ctx, loginPair.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if rotated.RefreshToken == loginPair.RefreshToken {
		t.Fatal("expected rotated refresh token")
	}
	// Old refresh must fail
	if _, err := svc.Refresh(ctx, loginPair.RefreshToken); err != domain.ErrUnauthorized {
		t.Fatalf("old refresh: %v", err)
	}

	if err := svc.Logout(ctx, rotated.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refresh(ctx, rotated.RefreshToken); err != domain.ErrUnauthorized {
		t.Fatalf("after logout: %v", err)
	}
}

func TestLoginBadPassword(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	_, _, err := svc.Register(ctx, "a@b.com", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = svc.Login(ctx, "a@b.com", "nope-nope")
	if err != domain.ErrInvalidCredentials {
		t.Fatalf("got %v", err)
	}
}
