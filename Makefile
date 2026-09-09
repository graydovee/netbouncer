GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "")
GIT_COMMIT := $(shell git rev-parse --short HEAD)
# 获取上一个tag（排除当前commit的tag）
PREV_TAG := $(shell git describe --tags --abbrev=0 HEAD~1 2>/dev/null || echo "v0.0.0")
# 检查当前commit是否有tag
CURRENT_COMMIT_HAS_TAG := $(shell git describe --exact-match --tags HEAD 2>/dev/null && echo "yes" || echo "no")
# 根据规则设置DOCKER_TAG
DOCKER_TAG := $(if $(filter yes,$(CURRENT_COMMIT_HAS_TAG)),$(GIT_TAG),$(PREV_TAG)-$(GIT_COMMIT))

.PHONY: all build-go build-go-embed run run-mock clean tidy docker-build docker-release build-web clean-web clean-go web-dev test

all: build-web build-go-embed

tidy:
	go mod tidy

# 构建React前端项目，产物复制到 web/（默认构建运行时读取）
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

# 本地构建（不嵌入前端，运行时需与 web/ 目录同在）
build-go: tidy
	CGO_ENABLED=1 go build -o bin/netbouncer main.go

# 构建内嵌前端的单文件二进制（需先 build-web）
build-go-embed: tidy build-web
	CGO_ENABLED=1 go build -tags embed -o bin/netbouncer main.go

# 使用配置文件运行
run:
	./bin/netbouncer -c config.yaml

# 本地调试（mock 防火墙，无需 root/CAP_NET_ADMIN）
run-mock:
	./bin/netbouncer -f mock

test:
	CGO_ENABLED=1 go test ./...

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
