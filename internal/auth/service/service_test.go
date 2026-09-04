package service_test

import (
	"context"
	"errors"
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

func TestRefreshReuseReturnsUnauthorized(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	_, pair, err := svc.Register(ctx, "reuse@example.com", "password123", "Reuse")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refresh(ctx, pair.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refresh(ctx, pair.RefreshToken); err != domain.ErrUnauthorized {
		t.Fatalf("reuse: got %v", err)
	}
}

func TestRefreshReuseRevokesFamily(t *testing.T) {
	svc := newTestService(t).WithReuseGrace(0)
	ctx := context.Background()
	_, pair, err := svc.Register(ctx, "family@example.com", "password123", "Family")
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := svc.Refresh(ctx, pair.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	_, other, err := svc.Login(ctx, "family@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	// Reuse of the already-rotated token after grace=0 revokes every session.
	if _, err := svc.Refresh(ctx, pair.RefreshToken); err != domain.ErrUnauthorized {
		t.Fatalf("reuse: got %v", err)
	}
	if _, err := svc.Refresh(ctx, rotated.RefreshToken); err != domain.ErrUnauthorized {
		t.Fatalf("rotated session should be wiped, got %v", err)
	}
	if _, err := svc.Refresh(ctx, other.RefreshToken); err != domain.ErrUnauthorized {
		t.Fatalf("other session should be wiped, got %v", err)
	}
}

func TestRefreshConcurrentRace(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	_, pair, err := svc.Register(ctx, "race@example.com", "password123", "Race")
	if err != nil {
		t.Fatal(err)
	}

	type result struct {
		tokens service.TokenPair
		err    error
	}
	ch := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() {
			tokens, err := svc.Refresh(ctx, pair.RefreshToken)
			ch <- result{tokens, err}
		}()
	}

	var ok, denied int
	for i := 0; i < 2; i++ {
		r := <-ch
		switch {
		case r.err == nil:
			ok++
			if r.tokens.RefreshToken == "" {
				t.Fatal("expected refresh token")
			}
		case errors.Is(r.err, domain.ErrUnauthorized):
			denied++
		default:
			t.Fatalf("unexpected err: %v", r.err)
		}
	}
	if ok != 1 || denied != 1 {
		t.Fatalf("ok=%d denied=%d", ok, denied)
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
