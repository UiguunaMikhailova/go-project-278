.PHONY: run build test lint check generate db-up db-down migrate migrate-down

run:
	go run main.go

build:
	go build -o bin/app .

test:
	go test -race ./...

lint:
	golangci-lint run

check: test lint

generate:
	sqlc generate

db-up:
	docker compose up -d db

db-down:
	docker compose down

migrate:
	goose -dir db/migrations postgres "$$DATABASE_URL" up

migrate-down:
	goose -dir db/migrations postgres "$$DATABASE_URL" down
