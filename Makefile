.PHONY: test fmt vet ci build hooks

GO ?= go
VERSION ?= $(shell node -p "require('./package.json').version" 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "")
LDFLAGS := -X repofalcon/internal/appinfo.Version=v$(VERSION) -X repofalcon/internal/appinfo.Commit=$(COMMIT)

test:
	$(GO) test ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

vet:
	$(GO) vet ./...

ci:
	@unformatted="$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*'))"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed on:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	$(GO) vet ./...
	$(GO) test ./...

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o bin/falcon ./cmd/falcon

hooks:
	git config core.hooksPath .githooks
	@echo "Git hooks installed (using .githooks/)"

