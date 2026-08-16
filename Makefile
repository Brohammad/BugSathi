# BugSathi — common developer commands
# Requires: Docker Compose, Go 1.24+ (system Go OR .tools/go)

COMPOSE := docker compose -f deploy/compose/docker-compose.yml
COMPOSE_PROD := docker compose -f deploy/compose/docker-compose.prod.yml --env-file .env.prod
LOCAL_GO := $(CURDIR)/.tools/go/bin/go

# Prefer project-local toolchain, then PATH.
ifeq ($(wildcard $(LOCAL_GO)),$(LOCAL_GO))
  GO := $(LOCAL_GO)
else
  GO ?= go
endif

.PHONY: help ensure-go bootstrap-go up down up-prod up-prod-obs down-prod logs ps tidy test test-race build build-images run-api run-worker health migrate fmt vet ci chaos-drill web-install web-dev web-build

help:
	@echo "Targets:"
	@echo "  make bootstrap-go Download Go into .tools/go (if missing)"
	@echo "  make up           Start local deps (Postgres, MinIO, Redpanda, obs)"
	@echo "  make down         Stop local deps"
	@echo "  make up-prod      Start production-like Compose (needs .env.prod)"
	@echo "  make up-prod-obs  Prod Compose + Prometheus/Grafana profile"
	@echo "  make down-prod    Stop production-like Compose"
	@echo "  make build-images Build api/worker/migrate Docker images"
	@echo "  make logs         Tail compose logs"
	@echo "  make tidy         go mod tidy"
	@echo "  make test         Run unit tests"
	@echo "  make build        Build api + worker + migrate"
	@echo "  make migrate      Apply SQL migrations"
	@echo "  make run-api      Run API on :8080 (needs migrate)"
	@echo "  make run-worker   Run worker health on :8081"
	@echo "  make web-install  npm install in web/"
	@echo "  make web-dev      Vite UI on :5173 (proxies API)"
	@echo "  make web-build    Production build of web/"
	@echo "  make health       Curl local health endpoints"
	@echo "  make chaos-drill  Postgres stop/start readiness drill (needs up-prod)"
	@echo "  make ci           fmt + vet + test"
	@echo ""
	@echo "Using GO=$(GO)"

ensure-go:
	@if ! command -v "$(GO)" >/dev/null 2>&1 && [ ! -x "$(GO)" ]; then \
		echo "Go not found."; \
		echo "Run:  make bootstrap-go"; \
		echo "Or install Go 1.24+ and ensure it is on PATH."; \
		exit 1; \
	fi
	@"$(GO)" version

# Downloads an official Go toolchain into .tools/go (gitignored).
bootstrap-go:
	@mkdir -p "$(CURDIR)/.tools"
	@if [ -x "$(LOCAL_GO)" ]; then \
		echo "Already present: $(LOCAL_GO)"; \
		"$(LOCAL_GO)" version; \
		exit 0; \
	fi
	@ARCH=$$(uname -m); \
	case "$$ARCH" in \
		arm64|aarch64) GOARCH=arm64 ;; \
		x86_64|amd64) GOARCH=amd64 ;; \
		*) echo "unsupported arch: $$ARCH"; exit 1 ;; \
	esac; \
	OS=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	VER=1.24.5; \
	TGZ="go$${VER}.$${OS}-$${GOARCH}.tar.gz"; \
	URL="https://go.dev/dl/$${TGZ}"; \
	echo "Downloading $${URL}"; \
	curl -fsSL -o "$(CURDIR)/.tools/go.tgz" "$${URL}" || curl -fsSL -A 'Mozilla/5.0' -o "$(CURDIR)/.tools/go.tgz" "https://dl.google.com/go/$${TGZ}"; \
	rm -rf "$(CURDIR)/.tools/go"; \
	tar -C "$(CURDIR)/.tools" -xzf "$(CURDIR)/.tools/go.tgz"; \
	rm -f "$(CURDIR)/.tools/go.tgz"; \
	"$(LOCAL_GO)" version

up:
	$(COMPOSE) up -d
	@echo "Postgres :5432  MinIO :9000/:9001  Redpanda :19092  Prometheus :9090  Grafana :3000  Jaeger :16686  OTLP :4318"

down:
	$(COMPOSE) down

up-prod:
	@test -f .env.prod || (echo "Copy .env.prod.example to .env.prod and set secrets"; exit 1)
	$(COMPOSE_PROD) up -d --build
	@echo "API :8080  Worker :8081  (prod compose)"

up-prod-obs:
	@test -f .env.prod || (echo "Copy .env.prod.example to .env.prod and set secrets"; exit 1)
	$(COMPOSE_PROD) --profile obs up -d --build
	@echo "API :8080  Worker :8081  Prometheus :9090  Grafana :3000"

down-prod:
	$(COMPOSE_PROD) --profile obs down

build-images:
	docker build -f deploy/docker/Dockerfile.api -t bugsathi/api:latest .
	docker build -f deploy/docker/Dockerfile.worker -t bugsathi/worker:latest .
	docker build -f deploy/docker/Dockerfile.migrate -t bugsathi/migrate:latest .

logs:
	$(COMPOSE) logs -f --tail=200

ps:
	$(COMPOSE) ps

tidy: ensure-go
	"$(GO)" mod tidy

test: ensure-go
	"$(GO)" test ./...

test-race: ensure-go
	"$(GO)" test -race ./...

build: ensure-go
	mkdir -p bin
	"$(GO)" build -o bin/api ./cmd/api
	"$(GO)" build -o bin/worker ./cmd/worker
	"$(GO)" build -o bin/migrate ./cmd/migrate

migrate: build
	./bin/migrate ./migrations

run-api: build
	HTTP_ADDR=:8080 ./bin/api

run-worker: build
	WORKER_HTTP_ADDR=:8081 ./bin/worker

health:
	@curl -sS -D - http://127.0.0.1:8080/healthz -o /tmp/bugsathi-api-health.json && echo && cat /tmp/bugsathi-api-health.json && echo
	@curl -sS -D - http://127.0.0.1:8081/healthz -o /tmp/bugsathi-worker-health.json && echo && cat /tmp/bugsathi-worker-health.json && echo

chaos-drill:
	@./scripts/chaos-drill.sh

web-install:
	cd web && npm install

web-dev:
	cd web && npm run dev

web-build:
	cd web && npm run build

fmt: ensure-go
	"$(GO)" fmt ./...

vet: ensure-go
	"$(GO)" vet ./...

ci: fmt vet test
