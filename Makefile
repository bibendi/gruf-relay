APP_NAME=gruf-relay
BUILD_DIR=build

.PHONY: build
build:
	# Create build directory
	mkdir -p $(BUILD_DIR)

	# Build the Go binary
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/gruf-relay

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

.PHONY: test
test:
	go test -v -cover -count=1 ./...
