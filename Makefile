SHELL := /bin/bash

GO ?= go
GOCACHE ?= $(CURDIR)/.cache/go-build
PACKAGES := ./...

.PHONY: help fmt fmt-check vet test cover cover-html tidy build build-cmd build-examples version ci verify clean

help:
	@echo "Available targets:"
	@echo "  make fmt        - format Go packages"
	@echo "  make fmt-check  - verify Go formatting"
	@echo "  make vet        - run go vet"
	@echo "  make test       - run unit tests"
	@echo "  make cover      - run tests with coverage output"
	@echo "  make cover-html - render coverage.html from coverage.out"
	@echo "  make tidy       - tidy go.mod/go.sum"
	@echo "  make build      - build with embedded version metadata"
	@echo "  make build-cmd  - build commands under cmd/"
	@echo "  make build-examples - compile examples under examples/"
	@echo "  make version    - print embedded build metadata"
	@echo "  make ci         - run the local CI quality gates"
	@echo "  make verify     - run the full local verification suite"
	@echo "  make clean      - remove local build artifacts"

fmt:
	@mkdir -p $(GOCACHE)
	@$(GO) fmt $(PACKAGES)

fmt-check:
	@files="$$(find . -type f -name '*.go' -not -path './.cache/*' -not -path './vendor/*')"; \
	if [ -n "$$files" ]; then \
		out="$$(gofmt -l $$files)"; \
		if [ -n "$$out" ]; then \
			echo "The following files need gofmt:"; \
			echo "$$out"; \
			exit 1; \
		fi; \
	fi

vet:
	@mkdir -p $(GOCACHE)
	@GOCACHE=$(GOCACHE) $(GO) vet $(PACKAGES)

test:
	@mkdir -p $(GOCACHE)
	@GOCACHE=$(GOCACHE) $(GO) test $(PACKAGES)

cover:
	@mkdir -p $(GOCACHE)
	@GOCACHE=$(GOCACHE) $(GO) test $(PACKAGES) -coverprofile=coverage.out
	@echo "coverage written to coverage.out"

cover-html: cover
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "coverage report written to coverage.html"

tidy:
	@$(GO) mod tidy

build:
	@$(MAKE) -f build/Makefile build

build-cmd:
	@$(MAKE) -f build/Makefile build-cmd

build-examples:
	@[ -d examples ] || { echo "no examples/ directory"; exit 0; }
	@mkdir -p bin/examples
	@for d in $$(find examples -maxdepth 1 -mindepth 1 -type d); do \
		name=$$(basename $$d); \
		echo "Building example $$name"; \
		GOCACHE=$(GOCACHE) $(GO) build -o bin/examples/$$name ./examples/$$name; \
	done

version:
	@$(MAKE) -f build/Makefile version

ci: fmt-check vet test build build-examples

verify: ci

clean:
	@rm -rf .cache bin coverage.out coverage.html
