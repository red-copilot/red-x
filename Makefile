.PHONY: build frontend test race vet lint verify compose-build compose-up clean

VERSION ?= 3.0.0

frontend:
	cd web && npm ci && npm run build

build: frontend
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o arl ./cmd/arl

test:
	go test ./...
	cd web && npm run typecheck && npm test

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
