.PHONY: build test cover lint tidy check

.DEFAULT_GOAL := build

build:
	go build ./...

test:
	go test ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

lint:
	golangci-lint run

tidy:
	go mod tidy

check: test lint
