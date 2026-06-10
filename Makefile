BINARY := mcp-guard
VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo unknown) -X main.date=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

.PHONY: all test lint build clean race

all: lint test build

test:
	go test -v ./...

race:
	go test -v -race ./...

lint:
	golangci-lint run ./...

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/mcp-guard

clean:
	rm -f $(BINARY)
