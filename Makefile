.PHONY: all build build-web build-widget build-go test clean run

BINARY_NAME=uploadgate
BIN_DIR=bin

all: build

build-web:
	@echo "Building React 19 web cockpit..."
	@cd web && npm install && npm run build

build-widget:
	@echo "Building client-side widget..."
	@cd widget && npm install && npm run build

build-go:
	@echo "Building Go executable..."
	@go build -trimpath -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/uploadgate

build: build-web build-widget build-go

test:
	@echo "Running tests..."
	@go test -v -race ./...

clean:
	@rm -rf $(BIN_DIR) web/dist widget/dist local_storage data

run: build-go
	@./$(BIN_DIR)/$(BINARY_NAME) serve --config config.example.yaml
