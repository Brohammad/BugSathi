package httpapi_test

import (
	"bytes"
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
