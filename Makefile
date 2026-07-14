.PHONY: build test run lint docker-build docker-run clean

build:
	go build -o bin/server ./cmd/rail-connectors

test:
	go test ./cmd/... ./internal/... -race -coverprofile=coverage.out -coverpkg=./cmd/...,./internal/...

run:
	go run ./cmd/rail-connectors

lint:
	golangci-lint run

docker-build:
	docker build -t ai-crypto-onramp/rail-connectors .

docker-run:
	docker run --rm -p 8080:8080 ai-crypto-onramp/rail-connectors

clean:
	rm -rf bin/ coverage.out
