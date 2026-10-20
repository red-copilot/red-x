.PHONY: build frontend test test-postgres race vet lint verify infra-up infra-down infra-config clean

VERSION ?= 3.0.0

frontend:
	cd web && npm ci && npm run build

build: frontend
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o arl ./cmd/arl

test: frontend
	go test ./...
	cd web && npm run typecheck && npm test

test-postgres:
	@test -n "$$ARL_TEST_POSTGRES_DSN" || (echo "set ARL_TEST_POSTGRES_DSN (PostgreSQL 18+)" >&2; exit 1)
	go test ./internal/store ./internal/modules/tasking

race: frontend
	go test -race ./...

vet: frontend
	go vet ./...

verify: frontend
	go mod verify
	go vet ./...
	go test ./...
	cd web && npm run lint && npm run typecheck && npm test

lint: frontend
	test -z "$$(gofmt -l cmd internal)"
	go vet ./...
	cd web && npm run lint && npm run typecheck

infra-up:
	docker compose -f deploy/compose.yaml up -d --wait

infra-down:
	docker compose -f deploy/compose.yaml down

infra-config:
	docker compose -f deploy/compose.yaml config >/dev/null

clean:
	go clean ./...
