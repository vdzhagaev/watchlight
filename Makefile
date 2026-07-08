ifneq (,$(wildcard ./.env))
    include .env
    export
endif

BINARY_NAME=watchlight

# seed/watch dev tools. Override on the CLI, e.g. `make seed HOST=example.com`.
HOST ?= example.com
NAME ?= $(HOST)

.PHONY: build run prepare fmt tidy seed watch

prepare: fmt tidy

fmt:
	go fmt ./...

tidy:
	go mod tidy

build: prepare
	mkdir -p bin
	go build -o bin/$(BINARY_NAME) ./cmd/server

run: build
	./bin/$(BINARY_NAME)

# seed one monitor (intrinsic ping + one HTTP check) into the DB
seed:
	go run ./cmd/seed "$(STORAGE_PATH)" "$(HOST)" "$(NAME)"

# stream check_results as the running server records them (Ctrl+C to stop)
watch:
	go run ./cmd/watch "$(STORAGE_PATH)"
