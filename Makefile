.PHONY: run test build docker-up docker-down demo

run:
	go run ./cmd/server

test:
	go test ./...

build:
	go build -o bin/certificate-inspector ./cmd/server

docker-up:
	docker compose up --build

docker-down:
	docker compose down

demo:
	./scripts/demo.sh
