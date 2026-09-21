BINARY     := svg2drawio
MODULE     := github.com/bcollard/svg2drawio
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE       ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"
BUILD_DIR  := ./bin

export CGO_ENABLED=0

.PHONY: build test vet fmt check install clean tidy snapshot release-check skill

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/$(BINARY)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w ./cmd ./internal *.go

check:
	@test -z "$$(gofmt -l ./cmd ./internal *.go)" || (echo "gofmt needed:"; gofmt -l ./cmd ./internal *.go; exit 1)
	go vet ./...
	go test ./...

install:
	go install $(LDFLAGS) ./cmd/$(BINARY)

# Install the Agent Skill from the working tree into Claude Code's skills dir.
skill: build
	$(BUILD_DIR)/$(BINARY) skill install --force

clean:
	rm -rf $(BUILD_DIR) dist

tidy:
	go mod tidy

# Local dry-run release (no publish). Requires goreleaser.
snapshot:
	goreleaser release --snapshot --clean

# Validate .goreleaser.yaml
release-check:
	goreleaser check
