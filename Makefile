# BugSathi — common developer commands
# Requires: Docker Compose, Go 1.24+

GO ?= go
COMPOSE := docker compose -f deploy/compose/docker-compose.yml
export PATH := $(CURDIR)/.tools/go/bin:$(PATH)

.PHONY: help up down logs ps tidy test test-race build run-api run-worker health migrate fmt vet ci

help:
	@echo "Targets:"
	@echo "  make up          Start Postgres, MinIO, Redpanda"
	@echo "  make down        Stop dependencies"
	@echo "  make logs        Tail compose logs"
	@echo "  make tidy        go mod tidy"
	@echo "  make test        Run unit tests"
	@echo "  make build       Build api + worker binaries"
	@echo "  make run-api     Run API on :8080"
	@echo "  make run-worker  Run worker health on :8081"
	@echo "  make health      Curl local health endpoints"
	@echo "  make migrate     Run SQL migrations (noop until schemas land)"
	@echo "  make ci          fmt + vet + test"

up:
	$(COMPOSE) up -d
	@echo "Postgres :5432  MinIO :9000/:9001  Redpanda Kafka :19092"

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f --tail=200

ps:
	$(COMPOSE) ps

tidy:
	$(GO) mod tidy

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

build:
	mkdir -p bin
	$(GO) build -o bin/api ./cmd/api
	$(GO) build -o bin/worker ./cmd/worker

run-api: build
	HTTP_ADDR=:8080 ./bin/api

run-worker: build
	WORKER_HTTP_ADDR=:8081 ./bin/worker

health:
	@curl -sS -D - http://127.0.0.1:8080/healthz -o /tmp/bugsathi-api-health.json && echo && cat /tmp/bugsathi-api-health.json && echo
	@curl -sS -D - http://127.0.0.1:8081/healthz -o /tmp/bugsathi-worker-health.json && echo && cat /tmp/bugsathi-worker-health.json && echo

migrate:
	@echo "No business migrations yet (Milestone 3+). Placeholder OK."
	@ls migrations >/dev/null 2>&1 || mkdir -p migrations

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

ci: fmt vet test
