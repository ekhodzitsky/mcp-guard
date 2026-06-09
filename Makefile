BINARY := mcp-guard
LDFLAGS := -s -w

.PHONY: all test lint build clean race

all: lint test build

test:
	go test -v -race ./...

race:
	go test -v -race ./...

lint:
	golangci-lint run ./...

build:
	CGO_ENABLED=1 go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/mcp-guard

clean:
	rm -f $(BINARY)
