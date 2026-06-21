.PHONY: all build test lint vet fmt clean install

BINARY := roost
GO ?= go
GOLANGCI_LINT ?= golangci-lint
COVERAGE_FILE := coverage.out

VERSION ?= dev
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)

LDFLAGS := -s -w \
	-X main.Version=$(VERSION) \
	-X main.BuildTime=$(BUILD_TIME) \
	-X main.GitCommit=$(GIT_COMMIT)

all: vet test lint build

build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	$(GO) test -race -coverprofile=$(COVERAGE_FILE) ./...

lint:
	$(GOLANGCI_LINT) run ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...
	goimports -w .

clean:
	$(GO) clean
	rm -f $(BINARY) $(COVERAGE_FILE) *.prof

install:
	$(GO) install -trimpath -ldflags "$(LDFLAGS)" .
