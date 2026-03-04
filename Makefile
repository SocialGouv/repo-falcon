.PHONY: test fmt vet ci build

GO ?= go

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
	$(GO) build -o bin/falcon ./cmd/falcon

