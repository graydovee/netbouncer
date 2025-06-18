GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "")
GIT_COMMIT := $(shell git rev-parse --short HEAD)
DOCKER_TAG := $(if $(GIT_TAG),$(GIT_TAG),$(if $(shell git describe --tags --abbrev=0 2>/dev/null),$(shell git describe --tags --abbrev=0)-$(GIT_COMMIT),v0.0.0-$(GIT_COMMIT)))

.PHONY: build run clean tidy docker-build docker-release
tidy:
	go mod tidy

build: tidy
	go build -o bin/netbouncer main.go

debug:
	./bin/netbouncer --debug

run:
	./bin/netbouncer

clean:
	rm -f bin/netbouncer

# Docker相关变量
DOCKER_IMAGE ?= graydovee/netbouncer
PLATFORMS ?= linux/amd64,linux/arm64
CURRENT_PLATFORM ?= $(shell go env GOOS)/$(shell go env GOARCH)

docker-build:
	@echo "Building local docker image $(DOCKER_IMAGE):$(DOCKER_TAG) for $(CURRENT_PLATFORM)"
	docker buildx build --platform $(CURRENT_PLATFORM) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		-t $(DOCKER_IMAGE):latest \
		--load .

docker-release:
	@echo "Building docker image $(DOCKER_IMAGE):$(DOCKER_TAG)"
	docker buildx build --platform $(PLATFORMS) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		-t $(DOCKER_IMAGE):latest \
		--push .
