.PHONY: help docker-up docker-down migrate-up migrate-down migrate-status run-api run-worker seed test build

help:
	@echo "StayHub Backend Commands:"
	@echo "  make docker-up       Start Postgres 15 and Redis 7"
	@echo "  make docker-down     Stop local containers"
	@echo "  make migrate-up      Apply all pending SQL migrations"
	@echo "  make migrate-down    Roll back the last SQL migration"
	@echo "  make migrate-status  Show applied migrations"
	@echo "  make run-api         Start the Gin HTTP API server"
	@echo "  make run-worker      Start the background timer & payout worker"
	@echo "  make seed            Seed sample accounts and listings (Lisbon, Goa, Kyoto)"
	@echo "  make test            Run all automated unit & integration tests"
	@echo "  make build           Compile binaries into ./bin"

docker-up:
	docker compose up -d

docker-down:
	docker compose down

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

migrate-status:
	go run ./cmd/migrate status

run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

seed:
	go run ./cmd/seed

test:
	go test -v -race ./...

build:
	mkdir -p bin
	go build -o bin/stayhub-api ./cmd/api
	go build -o bin/stayhub-worker ./cmd/worker
	go build -o bin/stayhub-migrate ./cmd/migrate
	go build -o bin/stayhub-seed ./cmd/seed
