ifneq (,$(wildcard ./.env))
    include .env
    export
endif

BINARY_NAME=watchlight

.PHONY: build run prepare fmt tidy

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
