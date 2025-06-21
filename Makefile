GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "")
GIT_COMMIT := $(shell git rev-parse --short HEAD)
# 获取上一个tag（排除当前commit的tag）
PREV_TAG := $(shell git describe --tags --abbrev=0 HEAD~1 2>/dev/null || echo "v0.0.0")
# 检查当前commit是否有tag
CURRENT_COMMIT_HAS_TAG := $(shell git describe --exact-match --tags HEAD 2>/dev/null && echo "yes" || echo "no")
# 根据规则设置DOCKER_TAG
DOCKER_TAG := $(if $(filter yes,$(CURRENT_COMMIT_HAS_TAG)),$(GIT_TAG),$(PREV_TAG)-$(GIT_COMMIT))

.PHONY: all build run clean tidy docker-build docker-release build-web clean-web clean-go web-dev

all: build-web build-go

tidy:
	go mod tidy

# 构建React前端项目
build-web:
	@echo "Building React frontend..."
	cd website && npm ci
	cd website && npm run build
	@echo "Copying built files to web directory..."
	rm -rf web
	mkdir -p web
	cp -r website/dist/* web/

# 开发模式运行前端项目
web-dev:
	@echo "Starting React frontend in development mode..."
	@echo "Backend URL: $(or $(VITE_BACKEND_URL),http://localhost:8080)"
	cd website && VITE_BACKEND_URL=$(or $(VITE_BACKEND_URL),http://localhost:8080) npm run dev

build-go: tidy
	go build -o bin/netbouncer main.go

debug:
	./bin/netbouncer --debug

run:
	./bin/netbouncer -c config.yaml

clean-web:
	rm -rf web
	rm -rf website/dist
	rm -rf website/node_modules

clean-go:
	rm -f bin/netbouncer

clean: clean-web clean-go

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
