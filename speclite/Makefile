.PHONY: build test test-verbose test-cover lint clean install

build:
	go build -o bin/speclite ./cmd/speclite

test:
	go test ./...

test-verbose:
	go test -v ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/ coverage.out coverage.html

install:
	go install ./cmd/speclite
