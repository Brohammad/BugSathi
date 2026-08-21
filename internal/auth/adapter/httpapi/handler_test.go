package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Brohammad/BugSathi/internal/auth/adapter/httpapi"
	"github.com/Brohammad/BugSathi/internal/auth/adapter/jwtmgr"
	"github.com/Brohammad/BugSathi/internal/auth/adapter/memory"
	"github.com/Brohammad/BugSathi/internal/auth/adapter/password"
	"github.com/Brohammad/BugSathi/internal/auth/service"
)

func TestRequireAccessAllowsSSEQueryToken(t *testing.T) {
	tm, err := jwtmgr.New("0123456789abcdef0123456789abcdef", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	svc := service.New(memory.NewUserRepo(), memory.NewRefreshRepo(), password.NewArgon2id(), tm, time.Hour)
	_, pair, err := svc.Register(context.Background(), "sse@example.com", "password123", "SSE")
	if err != nil {
		t.Fatal(err)
	}

	protected := httpapi.RequireAccess(svc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := httpapi.UserIDFromContext(r.Context()); !ok {
			t.Fatal("missing user")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	okReq := httptest.NewRequest(http.MethodGet, "/v1/projects/p/reports/r/events?access_token="+pair.AccessToken, nil)
	okRR := httptest.NewRecorder()
	protected.ServeHTTP(okRR, okReq)
	if okRR.Code != http.StatusNoContent {
		t.Fatalf("events query auth status=%d", okRR.Code)
	}

	denied := httptest.NewRequest(http.MethodGet, "/v1/projects/p?access_token="+pair.AccessToken, nil)
	deniedRR := httptest.NewRecorder()
	protected.ServeHTTP(deniedRR, denied)
	if deniedRR.Code != http.StatusUnauthorized {
		t.Fatalf("non-events query must not auth, got %d", deniedRR.Code)
	}
}

func TestAuthHTTPFlow(t *testing.T) {
	tm, err := jwtmgr.New("0123456789abcdef0123456789abcdef", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	svc := service.New(memory.NewUserRepo(), memory.NewRefreshRepo(), password.NewArgon2id(), tm, time.Hour)
	h := httpapi.NewHandler(svc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := []byte(`{"email":"u@example.com","password":"password123","name":"U"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("register status=%d body=%s", rr.Code, rr.Body.String())
	}

	var reg struct {
		Tokens struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &reg); err != nil {
		t.Fatal(err)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+reg.Tokens.AccessToken)
	meRR := httptest.NewRecorder()
	mux.ServeHTTP(meRR, meReq)
	if meRR.Code != http.StatusOK {
		t.Fatalf("me status=%d body=%s", meRR.Code, meRR.Body.String())
	}

	bad := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	badRR := httptest.NewRecorder()
	mux.ServeHTTP(badRR, bad)
	if badRR.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", badRR.Code)
	}
}

func TestRefreshReuseHTTP401(t *testing.T) {
	tm, err := jwtmgr.New("0123456789abcdef0123456789abcdef", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	svc := service.New(memory.NewUserRepo(), memory.NewRefreshRepo(), password.NewArgon2id(), tm, time.Hour)
	h := httpapi.NewHandler(svc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	regBody := []byte(`{"email":"r@example.com","password":"password123","name":"R"}`)
	regReq := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(regBody))
	regRR := httptest.NewRecorder()
	mux.ServeHTTP(regRR, regReq)
	if regRR.Code != http.StatusCreated {
		t.Fatalf("register status=%d", regRR.Code)
	}
	var reg struct {
		Tokens struct {
			RefreshToken string `json:"refresh_token"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(regRR.Body.Bytes(), &reg); err != nil {
		t.Fatal(err)
	}

	refreshBody, _ := json.Marshal(map[string]string{"refresh_token": reg.Tokens.RefreshToken})
	first := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", bytes.NewReader(refreshBody))
	firstRR := httptest.NewRecorder()
	mux.ServeHTTP(firstRR, first)
	if firstRR.Code != http.StatusOK {
		t.Fatalf("first refresh status=%d body=%s", firstRR.Code, firstRR.Body.String())
	}

	second := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", bytes.NewReader(refreshBody))
	secondRR := httptest.NewRecorder()
	mux.ServeHTTP(secondRR, second)
	if secondRR.Code != http.StatusUnauthorized {
		t.Fatalf("reuse refresh status=%d want 401 body=%s", secondRR.Code, secondRR.Body.String())
	}
}
