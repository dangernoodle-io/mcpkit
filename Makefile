.PHONY: build test cover lint tidy check docs docs-check

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

docs: ## regenerate per-package READMEs
	go run ./cmd/docsgen

docs-check: docs ## fail if generated READMEs drift
	git diff --exit-code -- '**/README.md' ':(exclude)README.md'
