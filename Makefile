BINARY     := code-review
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE       := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
MODULE     := github.com/LucasNav6/code-review-cli
LDFLAGS    := -s -w \
	-X $(MODULE)/internal/buildinfo.Version=$(VERSION) \
	-X $(MODULE)/internal/buildinfo.Commit=$(COMMIT) \
	-X $(MODULE)/internal/buildinfo.Date=$(DATE)

.PHONY: build
build:
	go build -ldflags '$(LDFLAGS)' -o bin/$(BINARY) ./cmd/code-review

.PHONY: install
install:
	go install -ldflags '$(LDFLAGS)' ./cmd/code-review

.PHONY: test
test:
	go test ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: fmt
fmt:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && echo 'gofmt found unformatted files' && exit 1)

.PHONY: check
check: fmt vet test

.PHONY: clean
clean:
	rm -rf bin dist
