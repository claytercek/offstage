BINARY    := offstage
BUILD_DIR := bin
CMD       := ./cmd/offstage

.PHONY: build test lint cover clean docs

build:
	go build -o $(BUILD_DIR)/$(BINARY) $(CMD)

test:
	go test ./...

lint:
	golangci-lint run ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report written to coverage.html"

docs:
	go run ./tools/gen-docs man

clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html man
