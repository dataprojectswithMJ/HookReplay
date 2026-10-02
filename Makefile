.PHONY: up down build run-api run-cli test dev

# Infrastructure
up:
	docker compose up -d

down:
	docker compose down

# Build
build:
	go build -o bin/hookreplay ./cmd/cli
	go build -o bin/hookreplay-api ./cmd/api

# Run (assumes `make up` already ran)
run-api:
	go run ./cmd/api

run-cli:
	go run ./cmd/cli

# Tunnel in one shell, run in another:
#   go run ./cmd/cli tunnel --port 3000
#   TUNNEL_URL=<printed-url> go run ./cmd/cli run

test:
	go test ./...

# Dashboard
dev:
	cd web && npm install && npm run dev
