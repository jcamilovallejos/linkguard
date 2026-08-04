.PHONY: build run test fmt vet tidy lint up down logs

build:
	go build -o bin/linkguard ./cmd/server

run:
	go run ./cmd/server

test:
	go test -race -cover ./...

fmt:
	gofmt -l -w .

vet:
	go vet ./...

tidy:
	go mod tidy

lint:
	docker run --rm -v $(PWD):/app -w /app golangci/golangci-lint:latest \
		golangci-lint run

up:
	docker compose up --build

down:
	docker compose down -v

logs:
	docker compose logs -f
