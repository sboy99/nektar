.PHONY: build run test lint deps infra-up infra-down clean

BINARY := bin/nektar
CONFIG := configs/config.yaml

build:
	go build -o $(BINARY) ./cmd/nektar

run: build
	./$(BINARY) -config $(CONFIG)

test:
	go test ./...

lint:
	go vet ./...

deps:
	go mod tidy

infra-up:
	docker compose -f deployments/docker-compose.yaml up -d

infra-down:
	docker compose -f deployments/docker-compose.yaml down

clean:
	rm -rf bin/
