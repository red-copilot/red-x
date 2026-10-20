.PHONY: build frontend test test-postgres race vet lint verify compose-build compose-up clean

VERSION ?= 3.0.0

frontend:
	cd web && npm ci && npm run build

build: frontend
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o arl ./cmd/arl

test:
	go test ./...
	cd web && npm run typecheck && npm test

test-postgres:
	@test -n "$$ARL_TEST_POSTGRES_DSN" || (echo "set ARL_TEST_POSTGRES_DSN (PostgreSQL 18+)" >&2; exit 1)
	go test ./internal/store ./internal/modules/tasking

race:
	go test -race ./...

vet:
	go vet ./...

verify:
	go mod verify
	go vet ./...
	go test ./...
	cd web && npm run lint && npm run typecheck && npm test && npm run build

lint:
	test -z "$$(gofmt -l cmd internal)"
	go vet ./...
	cd web && npm run lint && npm run typecheck

compose-build:
	docker compose -f deploy/compose.yaml build

compose-up:
	docker compose -f deploy/compose.yaml up -d

clean:
	go clean ./...
