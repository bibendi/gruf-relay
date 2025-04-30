APP_NAME=gruf-relay
BUILD_DIR=build
GEM_DIR=gem

VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "0.1.0-dev")
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

.PHONY: build-gems
build-gems: $(PLATFORMS:%=build-gem-%)

.PHONY: build-gem-linux-amd64
build-gem-linux-amd64:
	mkdir -p $(GEM_DIR)/exe
	GOOS=linux GOARCH=amd64 go build \
		-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)" \
		-o $(GEM_DIR)/exe/$(APP_NAME)-linux-amd64 \
		./cmd/gruf-relay
	cd $(GEM_DIR) && GEM_VERSION=$(GEM_VERSION) PLATFORM=x86_64-linux BINARY_NAME=$(APP_NAME)-linux-amd64 gem build -o $(APP_NAME)-$(GEM_VERSION)-linux-amd64.gem

.PHONY: build-gem-linux-arm64
build-gem-linux-arm64:
	mkdir -p $(GEM_DIR)/exe
	GOOS=linux GOARCH=arm64 go build \
		-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)" \
		-o $(GEM_DIR)/exe/$(APP_NAME)-linux-arm64 \
		./cmd/gruf-relay
	cd $(GEM_DIR) && GEM_VERSION=$(GEM_VERSION) PLATFORM=arm64-linux BINARY_NAME=$(APP_NAME)-linux-arm64 gem build -o $(APP_NAME)-$(GEM_VERSION)-linux-arm64.gem

.PHONY: build-gem-darwin-amd64
build-gem-darwin-amd64:
	mkdir -p $(GEM_DIR)/exe
	GOOS=darwin GOARCH=amd64 go build \
		-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)" \
		-o $(GEM_DIR)/exe/$(APP_NAME)-darwin-amd64 \
		./cmd/gruf-relay
	cd $(GEM_DIR) && GEM_VERSION=$(GEM_VERSION) PLATFORM=x86_64-darwin BINARY_NAME=$(APP_NAME)-darwin-amd64 gem build -o $(APP_NAME)-$(GEM_VERSION)-darwin-amd64.gem

.PHONY: build-gem-darwin-arm64
build-gem-darwin-arm64:
	mkdir -p $(GEM_DIR)/exe
	GOOS=darwin GOARCH=arm64 go build \
		-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)" \
		-o $(GEM_DIR)/exe/$(APP_NAME)-darwin-arm64 \
		./cmd/gruf-relay
	cd $(GEM_DIR) && GEM_VERSION=$(GEM_VERSION) PLATFORM=arm64-darwin BINARY_NAME=$(APP_NAME)-darwin-arm64 gem build -o $(APP_NAME)-$(GEM_VERSION)-darwin-arm64.gem

.PHONY: publish-gems
publish-gems: build-gems
	@echo "Publishing gems to RubyGems.org"
	cd $(GEM_DIR) && for gemfile in $(APP_NAME)-$(GEM_VERSION)-*.gem; do \
		echo "Publishing $$gemfile..."; \
		gem push $$gemfile; \
	done

.PHONY: publish-gem-linux-amd64
publish-gem-linux-amd64: build-gem-linux-amd64
	cd $(GEM_DIR) && gem push $(APP_NAME)-$(GEM_VERSION)-linux-amd64.gem

.PHONY: publish-gem-linux-arm64
publish-gem-linux-arm64: build-gem-linux-arm64
	cd $(GEM_DIR) && gem push $(APP_NAME)-$(GEM_VERSION)-linux-arm64.gem

.PHONY: publish-gem-darwin-amd64
publish-gem-darwin-amd64: build-gem-darwin-amd64
	cd $(GEM_DIR) && gem push $(APP_NAME)-$(GEM_VERSION)-darwin-amd64.gem

.PHONY: publish-gem-darwin-arm64
publish-gem-darwin-arm64: build-gem-darwin-arm64
	cd $(GEM_DIR) && gem push $(APP_NAME)-$(GEM_VERSION)-darwin-arm64.gem

.PHONY: build-docker
build-docker: build-gems
	@echo "Building Docker image for $(APP_NAME):$(VERSION)"
	docker build \
		-t $(APP_NAME):$(VERSION) \
		-f example/kubernetes/Dockerfile .

.PHONY: run-docker
run-docker:
	docker run -p 8080:8080 -p 9394:9394 -p 5555:5555 \
		-it --rm \
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

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)
	rm -rf $(GEM_DIR)/exe
	rm -f $(GEM_DIR)/*.gem

.PHONY: test
test:
	go test -v -cover -count=1 ./...
