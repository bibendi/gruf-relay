APP_NAME=gruf-relay
BUILD_DIR=build
GEM_DIR=gem

VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "0.1.0-dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GEM_VERSION ?= $(VERSION)

PLATFORMS = linux-amd64 linux-arm64 darwin-amd64 darwin-arm64

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

.PHONY: build-binary
build-binary:
	mkdir -p $(BUILD_DIR)
	GOOS=$(BUILD_OS) GOARCH=$(BUILD_ARCH) go build \
		-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)" \
		-o $(BUILD_DIR)/$(APP_NAME)-$(BUILD_OS)-$(BUILD_ARCH) \
		./cmd/gruf-relay

.PHONY: build-binaries
build-binaries:
	@echo "Building binaries for all platforms..."
	$(MAKE) build-binary-linux-amd64
	$(MAKE) build-binary-linux-arm64
	$(MAKE) build-binary-darwin-amd64
	$(MAKE) build-binary-darwin-arm64
	@echo "All binaries built successfully."

.PHONY: build-binary-linux-amd64
build-binary-linux-amd64: BUILD_OS = linux
build-binary-linux-amd64: BUILD_ARCH = amd64
build-binary-linux-amd64: build-binary

.PHONY: build-binary-linux-arm64
build-binary-linux-arm64: BUILD_OS = linux
build-binary-linux-arm64: BUILD_ARCH = arm64
build-binary-linux-arm64: build-binary

.PHONY: build-binary-darwin-amd64
build-binary-darwin-amd64: BUILD_OS = darwin
build-binary-darwin-amd64: BUILD_ARCH = amd64
build-binary-darwin-amd64: build-binary

.PHONY: build-binary-darwin-arm64
build-binary-darwin-arm64: BUILD_OS = darwin
build-binary-darwin-arm64: BUILD_ARCH = arm64
build-binary-darwin-arm64: build-binary

.PHONY: build-gems
build-gems: build-binaries
	@echo "Building gems for all platforms..."
	$(MAKE) build-gem-linux-amd64
	$(MAKE) build-gem-linux-arm64
	$(MAKE) build-gem-darwin-amd64
	$(MAKE) build-gem-darwin-arm64
	@echo "All gems built successfully."

.PHONY: build-gem-linux-amd64
build-gem-linux-amd64:
	mkdir -p $(GEM_DIR)/exe
	cp $(BUILD_DIR)/$(APP_NAME)-linux-amd64 $(GEM_DIR)/exe/$(APP_NAME)-linux-amd64
	cd $(GEM_DIR) && GEM_VERSION=$(GEM_VERSION) PLATFORM=x86_64-linux BINARY_NAME=$(APP_NAME)-linux-amd64 gem build -o pkg/$(APP_NAME)-$(GEM_VERSION)-linux-amd64.gem

.PHONY: build-gem-linux-arm64
build-gem-linux-arm64:
	mkdir -p $(GEM_DIR)/exe
	cp $(BUILD_DIR)/$(APP_NAME)-linux-arm64 $(GEM_DIR)/exe/$(APP_NAME)-linux-arm64
	cd $(GEM_DIR) && GEM_VERSION=$(GEM_VERSION) PLATFORM=arm64-linux BINARY_NAME=$(APP_NAME)-linux-arm64 gem build -o pkg/$(APP_NAME)-$(GEM_VERSION)-linux-arm64.gem

.PHONY: build-gem-darwin-amd64
build-gem-darwin-amd64:
	mkdir -p $(GEM_DIR)/exe
	cp $(BUILD_DIR)/$(APP_NAME)-darwin-amd64 $(GEM_DIR)/exe/$(APP_NAME)-darwin-amd64
	cd $(GEM_DIR) && GEM_VERSION=$(GEM_VERSION) PLATFORM=x86_64-darwin BINARY_NAME=$(APP_NAME)-darwin-amd64 gem build -o pkg/$(APP_NAME)-$(GEM_VERSION)-darwin-amd64.gem

.PHONY: build-gem-darwin-arm64
build-gem-darwin-arm64:
	mkdir -p $(GEM_DIR)/exe
	cp $(BUILD_DIR)/$(APP_NAME)-darwin-arm64 $(GEM_DIR)/exe/$(APP_NAME)-darwin-arm64
	cd $(GEM_DIR) && GEM_VERSION=$(GEM_VERSION) PLATFORM=arm64-darwin BINARY_NAME=$(APP_NAME)-darwin-arm64 gem build -o pkg/$(APP_NAME)-$(GEM_VERSION)-darwin-arm64.gem

.PHONY: publish-gems
publish-gems:
	@echo "Publishing gems for all platforms..."
	$(MAKE) publish-gem-linux-amd64
	$(MAKE) publish-gem-linux-arm64
	$(MAKE) publish-gem-darwin-amd64
	$(MAKE) publish-gem-darwin-arm64
	@echo "All gems published successfully."

.PHONY: publish-gem-linux-amd64
publish-gem-linux-amd64:
	cd $(GEM_DIR)/pkg && gem push $(APP_NAME)-$(GEM_VERSION)-linux-amd64.gem

.PHONY: publish-gem-linux-arm64
publish-gem-linux-arm64:
	cd $(GEM_DIR)/pkg && gem push $(APP_NAME)-$(GEM_VERSION)-linux-arm64.gem

.PHONY: publish-gem-darwin-amd64
publish-gem-darwin-amd64:
	cd $(GEM_DIR)/pkg && gem push $(APP_NAME)-$(GEM_VERSION)-darwin-amd64.gem

.PHONY: publish-gem-darwin-arm64
publish-gem-darwin-arm64:
	cd $(GEM_DIR)/pkg && gem push $(APP_NAME)-$(GEM_VERSION)-darwin-arm64.gem

.PHONY: build-docker
build-docker: build-gems
	@echo "Building Docker image for $(APP_NAME):$(VERSION)"
	docker build \
		-t $(APP_NAME):$(VERSION) \
		-f example/Dockerfile .

.PHONY: run
run:
	cd example && ../$(BUILD_DIR)/gruf-relay

.PHONY: run-docker
run-docker:
	docker run -p 8080:8080 -p 9394:9394 -p 5555:5555 \
		--rm \
		--name $(APP_NAME) $(APP_NAME):$(VERSION) $(cmd)

.PHONY: k8s-apply-gruf-relay
k8s-apply-gruf-relay:
	VERSION=$(VERSION) envsubst < example/kubernetes/gruf-relay-deployment.yaml | kubectl apply -f -
	VERSION=$(VERSION) envsubst < example/kubernetes/gruf-relay-service.yaml | kubectl apply -f -

.PHONY: k8s-apply-gruf
k8s-apply-gruf:
	VERSION=$(VERSION) GRUF_BACKLOG_PATCH= envsubst < example/kubernetes/gruf-deployment.yaml | kubectl apply -f -
	VERSION=$(VERSION) envsubst < example/kubernetes/gruf-service.yaml | kubectl apply -f -

.PHONY: k8s-apply-gruf-with-patch
k8s-apply-gruf-with-patch:
	VERSION=$(VERSION) GRUF_BACKLOG_PATCH=true envsubst < example/kubernetes/gruf-deployment.yaml | kubectl apply -f -
	VERSION=$(VERSION) envsubst < example/kubernetes/gruf-service.yaml | kubectl apply -f -

.PHONY: k8s-delete
k8s-delete:
	VERSION=$(VERSION) envsubst < example/kubernetes/gruf-relay-service.yaml | kubectl delete --ignore-not-found -f -
	VERSION=$(VERSION) envsubst < example/kubernetes/gruf-relay-deployment.yaml | kubectl delete --ignore-not-found -f -
	VERSION=$(VERSION) envsubst < example/kubernetes/gruf-service.yaml | kubectl delete --ignore-not-found -f -
	VERSION=$(VERSION) envsubst < example/kubernetes/gruf-deployment.yaml | kubectl delete --ignore-not-found -f -

.PHONY: k6-run
k6-run:
	cd example && k6 run --log-output none k6.js

.PHONY: lint
lint:
	golangci-lint run

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)
	rm -rf $(GEM_DIR)/exe
	rm -f $(GEM_DIR)/pkg/*.gem

.PHONY: test
test:
	go test -v -cover -count=1 ./internal

.PHONY: test-e2e
test-e2e:
	go test -v -cover -count=1 ./e2e
