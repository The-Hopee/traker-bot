.PHONY: build run dev docker-build docker-up docker-down docker-logs migrate test clean

build:
	go build -o bin/bot ./cmd/bot

run: build
	./bin/bot

dev:
	go run ./cmd/bot

docker-build:
	docker compose up --build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f bot

docker-restart:
	docker compose restart bot

migrate:
	psql $(DATABASE_URL) -f migrations/001_init.sql

test:
	go test -v ./...

clean:
	rm -rf bin/
	docker-compose down -v

deploy:
	./scripts/deploy.sh