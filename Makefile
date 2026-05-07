.PHONY: build run test migrate-up migrate-down sqlc-generate docker-up docker-down docker-logs seed lint

build:
	go build -o server ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./...

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down 1

sqlc-generate:
	go tool sqlc generate

docker-up:
	docker compose -f docker/docker-compose.yml up -d

docker-down:
	docker compose -f docker/docker-compose.yml down

docker-logs:
	docker compose -f docker/docker-compose.yml logs -f app

seed:
	go run ./scripts/seed.go

lint:
	go vet ./...
