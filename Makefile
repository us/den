BINARY := den
VERSION ?= dev
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)"

.PHONY: build test test-integration test-sdk e2e-network lint clean dashboard release docker

build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/den

test:
	go test ./internal/... -short -v

test-integration:
	go test -tags integration ./internal/... ./tests/... -run TestIntegration -v

test-sdk:
	cd sdk/typescript && bun test
	cd sdk/python && uv run python -m pytest

# Machine-checkable end-to-end network proof (real den binary + real Docker).
# Set DEN_E2E_LOCAL_NATIVE=1 ONLY on native co-resident Linux for leg D.
e2e-network:
	./scripts/e2e-network.sh

lint:
	golangci-lint run ./...

fmt:
	gofmt -s -w .

vet:
	go vet ./...

dashboard:
	cd dashboard-ui && bun install && bun run build
	cp -r dashboard-ui/dist/* internal/dashboard/dist/

clean:
	rm -rf bin/ internal/dashboard/dist/*.js internal/dashboard/dist/*.css

release:
	goreleaser release --clean

docker:
	docker build -t den/den:latest .

docker-image:
	docker build -t den/default:latest images/default/

run: build
	./bin/$(BINARY) serve

all: lint test build
