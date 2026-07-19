APP := vefr
GO ?= go
BIN_DIR := bin
IMAGE ?= vefr:local

.PHONY: all fmt fmt-check test race vet check build clean run docker-build docker-check

all: check build

fmt:
	@gofmt -w $$(find . -type f -name '*.go' -not -path './vendor/*')

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo 'Go files need formatting:'; gofmt -l .; exit 1)

test:
	$(GO) test ./...

race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

check: fmt-check vet test

build:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -o $(BIN_DIR)/$(APP) ./cmd/proxy

run:
	$(GO) run ./cmd/proxy -config $${VEFR_CONFIG:-config.toml}

docker-build:
	docker build --tag $(IMAGE) .

docker-check:
	docker run --rm $(IMAGE) -h

clean:
	rm -rf $(BIN_DIR)
