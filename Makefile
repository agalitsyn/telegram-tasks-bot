export GOBIN := $(CURDIR)/bin
export PATH := $(GOBIN):$(PATH)

GIT_TAG := $(shell git describe --tags --always --abbrev=0)

BUILD_ARGS ?= -ldflags \
	"-X github.com/agalitsyn/telegram-tasks-bot/version.Tag=$(GIT_TAG)"

ifneq (,$(wildcard ./.env))
    include .env
    export
endif

.PHONY: build
build: $(GOBIN)
	go build -mod=vendor -v $(BUILD_ARGS) -o $(GOBIN) ./cmd/...

.PHONY: clean
clean:
	rm -rf "$(GOBIN)"

$(GOBIN):
	mkdir -p $(GOBIN)

.PHONY: vendor
vendor:
		go mod tidy
		go mod vendor

include bin-deps.mk

.PHONY: start
start:
	go run -mod=vendor $(CURDIR)/cmd/bot

.PHONY: test-short
test-short:
	go test -v -race -short ./...

.PHONY: test
test:
	go test -v -race ./...

.PHONY: fmt
fmt: $(GOLANGCI_BIN)
	$(GOLANGCI_BIN) run --fix ./...

.PHONY: lint
lint: $(GOLANGCI_BIN)
	@go version
	@$(GOLANGCI_BIN) version
	$(GOLANGCI_BIN) run ./...

.PHONY: generate
generate:
	go generate ./...

.PHONY: db-migrate
db-migrate:
	go run -mod=vendor $(CURDIR)/cmd/bot --migrate

.PHONY: db-reset
db-reset:
	mv db.sqlite3 db-prev.sqlite3
	go run -mod=vendor $(CURDIR)/cmd/bot --migrate

.PHONY: db-populate
db-populate:
	sqlite3 db.sqlite3 < fixtures/fixtures.sql
