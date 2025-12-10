BINARY := bin/snow9s
GO ?= go

.PHONY: build test run clean install

build:
	$(GO) build -o $(BINARY) ./cmd/snow9s

test:
	$(GO) test ./...

run: build
	./$(BINARY)

clean:
	rm -rf bin

install:
	$(GO) install ./cmd/snow9s
