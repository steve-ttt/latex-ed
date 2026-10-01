.PHONY: all test build run clean

BINARY_NAME=latex-editor
PORT?=8081

all: test build

test:
	go test -v -race ./...

build:
	go build -o bin/$(BINARY_NAME) ./cmd/latex-editor

run: build
	./bin/$(BINARY_NAME) -port $(PORT)

clean:
	rm -rf bin/ .latex-cache/
