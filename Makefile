BIN_DIR := bin
BIN := $(BIN_DIR)/gofetch
GO := go

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build clean fmt vet test tidy install

all: build

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

build: $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/mcp-fetch-go

clean:
	rm -rf $(BIN_DIR)

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

test:
	$(GO) test -race -cover ./...

tidy:
	$(GO) mod tidy

install:
	CGO_ENABLED=0 $(GO) install ./cmd/mcp-fetch-go