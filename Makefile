APP_NAME=gruf-relay
BUILD_DIR=build

.PHONY: build
build:
	# Create build directory
	mkdir -p $(BUILD_DIR)

	# Build the Go binary for the current platform
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/gruf-relay

.PHONY: build-amd64
build-amd64:
	# Create build directory
	mkdir -p $(BUILD_DIR)

	# Build the Go binary for amd64 Linux
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(APP_NAME)-amd64 ./cmd/gruf-relay

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

.PHONY: test
test:
	go test -v -cover -count=1 ./...
