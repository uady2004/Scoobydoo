# ─────────────────────────────────────────────────────────────────────────────
# TikTok Clone — Root Makefile
# ─────────────────────────────────────────────────────────────────────────────

.PHONY: help up down restart logs build build-all test lint tidy \
        infra-up infra-down db-migrate db-seed \
        proto go-work-sync

# Default target
help:
	@echo ""
	@echo "TikTok Clone — available make targets:"
	@echo ""
	@echo "  Dev lifecycle:"
	@echo "    make up              Start all services (infra + microservices)"
	@echo "    make infra-up        Start infrastructure only (postgres, redis, kafka…)"
	@echo "    make down            Stop and remove all containers"
	@echo "    make restart svc=X   Restart a single service  (e.g. make restart svc=auth-service)"
	@echo "    make logs svc=X      Tail logs for a service"
	@echo ""
	@echo "  Build:"
	@echo "    make build svc=X     Build a single service Docker image"
	@echo "    make build-all       Build all service images"
	@echo ""
	@echo "  Database:"
	@echo "    make db-migrate      Run all SQL migrations against postgres"
	@echo "    make db-seed         Seed postgres with sample data"
	@echo ""
	@echo "  Go:"
	@echo "    make test            Run all Go tests across the workspace"
	@echo "    make lint            Run golangci-lint on all modules"
	@echo "    make tidy            Run go mod tidy on every module"
	@echo "    make go-work-sync    Sync go.work.sum"
	@echo ""

# ─── Dev lifecycle ────────────────────────────────────────────────────────────

up:
	docker compose up -d

infra-up:
	docker compose up -d postgres redis zookeeper kafka elasticsearch minio clickhouse

down:
	docker compose down --remove-orphans

restart:
ifndef svc
	$(error Usage: make restart svc=<service-name>)
endif
	docker compose restart $(svc)

logs:
ifndef svc
	$(error Usage: make logs svc=<service-name>)
endif
	docker compose logs -f --tail=100 $(svc)

# ─── Build ────────────────────────────────────────────────────────────────────

build:
ifndef svc
	$(error Usage: make build svc=<service-name>)
endif
	docker compose build $(svc)

build-all:
	docker compose build

# ─── Database ─────────────────────────────────────────────────────────────────

db-migrate:
	@echo "Running migrations…"
	@for f in database/postgres/*.sql; do \
		echo "  Applying $$f"; \
		docker compose exec -T postgres psql -U postgres -d tiktok -f /dev/stdin < $$f; \
	done
	@echo "Migrations complete."

db-seed:
	@echo "Seeding database…"
	bash scripts/seed.sh
	@echo "Seeding complete."

# ─── Go ───────────────────────────────────────────────────────────────────────

test:
	go test ./... -count=1 -race -timeout 120s

lint:
	@which golangci-lint > /dev/null 2>&1 || \
		(echo "Installing golangci-lint…" && \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

tidy:
	@for dir in \
		shared \
		backend/api-gateway \
		database/redis \
		services/admin-service \
		services/ads-service \
		services/analytics-service \
		services/auth-service \
		services/ecommerce-service \
		services/feed-service \
		services/interaction-service \
		services/livestream-service \
		services/messaging-service \
		services/moderation-service \
		services/notification-service \
		services/payment-service \
		services/recommendation-service \
		services/reporting-service \
		services/search-service \
		services/social-graph-service \
		services/user-service \
		services/video-service \
		services/wallet-service; do \
		echo "  tidy: $$dir"; \
		(cd $$dir && GOWORK=off go mod tidy); \
	done

go-work-sync:
	go work sync

# ─── Proto ────────────────────────────────────────────────────────────────────

proto:
	@which protoc > /dev/null 2>&1 || (echo "protoc not found — install protobuf-compiler" && exit 1)
	@for f in shared/protobuf/*.proto; do \
		echo "  Compiling $$f"; \
		protoc --go_out=. --go-grpc_out=. $$f; \
	done
