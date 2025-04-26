APP_NAME=gruf-relay
BUILD_DIR=build

VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

.PHONY: build
build:
	# Create build directory
	mkdir -p $(BUILD_DIR)

	@echo "Building gruf-relay version $(VERSION), commit $(COMMIT), built on $(BUILD_DATE)"

	# Build the Go binary for the current platform
	go build \
		-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)" \
		-o $(BUILD_DIR)/$(APP_NAME) \
		./cmd/gruf-relay

.PHONY: build-amd64
build-amd64:
	GOOS=linux GOARCH=amd64 $(MAKE) build APP_NAME=$(APP_NAME)-amd64

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)

.PHONY: test
test:
	go test -v -cover -count=1 ./...
