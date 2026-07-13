.PHONY: build run test test-integration lint hooks deps infra-up infra-down clean

BINARY := bin/nektar
CONFIG := configs/config.yaml
POSTGRES_DSN ?= postgres://postgres:postgres@localhost:5432/nektar?sslmode=disable
REDIS_ADDR ?= localhost:6379

build:
	go build -o $(BINARY) ./cmd/nektar

build-gmail-auth:
	go build -o bin/nektar-gmail-auth cmd/nektar-gmail-auth/main.go

run: build
	./$(BINARY) -config $(CONFIG)

test:
	go test ./...

test-integration:
	NEKTAR_POSTGRES_DSN='$(POSTGRES_DSN)' NEKTAR_REDIS_ADDR='$(REDIS_ADDR)' \
		go test ./internal/adapters/postgres/... ./internal/adapters/redis/... -count=1 -v

lint:
	gofmt -l . | (! grep .)
	go vet ./...

hooks:
	git config core.hooksPath .githooks

deps:
	go mod tidy

infra-up:
	docker compose -f deployments/docker-compose.yaml up -d

infra-down:
	docker compose -f deployments/docker-compose.yaml down

clean:
	rm -rf bin/
