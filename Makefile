.PHONY: all build test lint vet fmt clean install

BINARY := roost
GO ?= go
GOLANGCI_LINT ?= golangci-lint
COVERAGE_FILE := coverage.out

all: vet test lint build

build:
	$(GO) build -o $(BINARY) .

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
	$(GO) install .
