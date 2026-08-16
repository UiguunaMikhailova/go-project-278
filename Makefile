.PHONY: run build test lint check

run:
	go run main.go

build:
	go build -o bin/app .

test:
	go test -race ./...

lint:
	golangci-lint run

check: test lint
