VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X 'corelab/pkg/version.Version=$(VERSION)' -X 'corelab/pkg/version.Commit=$(COMMIT)' -X 'corelab/pkg/version.Date=$(DATE)'

build:
	go build -ldflags "$(LDFLAGS)" ./...

# If you have commands under cmd/<name>, build an example binary:
build-cmd:
	@[ -d cmd ] || { echo "no cmd/ directory"; exit 0; }
	@mkdir -p bin
	@for d in $$(find cmd -maxdepth 1 -mindepth 1 -type d); do \
		name=$$(basename $$d); \
		echo "Building $$name"; \
		go build -ldflags "$(LDFLAGS)" -o bin/$$name ./cmd/$$name; \
	done

.PHONY: build version

version:
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Date:    $(DATE)"
