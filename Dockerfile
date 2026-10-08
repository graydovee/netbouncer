# Node.js构建阶段 - 构建React前端
# 前端产物是静态文件，与运行架构无关：固定在本机构建平台跑一次，两架构共用
FROM --platform=$BUILDPLATFORM harbor.graydove.cn/library/node:18-alpine AS frontend-builder

WORKDIR /app/frontend

# 复制package文件
COPY website/package*.json ./

# 安装所有依赖（包括开发依赖；npm 走国内镜像，proxy.golang.org/registry.npmjs.org 直连不稳定）
RUN --mount=type=cache,target=/root/.npm \
    npm config set registry https://registry.npmmirror.com && npm ci

# 复制前端源代码
COPY website/ .

# 构建前端项目
RUN npm run build

# Go 构建阶段：双架构分别在原生 CI runner 上编译，避免 QEMU/buildx 构建
FROM --platform=$BUILDPLATFORM harbor.graydove.cn/library/golang:1.25.7-bookworm AS builder

ARG TARGETOS
ARG TARGETARCH

# CI uses one native runner for each architecture. Avoid downloading the unused
# cross toolchain and the second architecture's libc/pcap packages.
RUN sed -i 's|http://deb.debian.org|https://mirrors.aliyun.com|g' /etc/apt/sources.list.d/debian.sources \
    && apt-get update \
    && apt-get install -y --no-install-recommends libpcap0.8-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Go 模块代理走国内（proxy.golang.org 直连不稳定）
ENV GOPROXY=https://goproxy.cn,direct

# 复制go mod文件
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# 复制源代码
COPY . .

# 复制前端构建产物，嵌入二进制实现单文件部署
COPY --from=frontend-builder /app/frontend/dist ./pkg/web/dist

# Run the complete test suite with the native Linux toolchain before publishing either architecture.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go test -race ./...

# Refuse emulated/cross builds: publish each architecture from its native runner.
RUN set -eu; \
    BUILD_HOST_ARCH="$(dpkg --print-architecture)"; \
    test "${TARGETARCH:-$BUILD_HOST_ARCH}" = "$BUILD_HOST_ARCH"; \
    CGO_ENABLED=1 GOOS=${TARGETOS:-linux} GOARCH=$BUILD_HOST_ARCH go build -tags embed -o netbouncer main.go

# 运行阶段
FROM harbor.graydove.cn/library/ubuntu:22.04

# 安装运行时依赖（apt 走国内镜像；
# 基础镜像尚无 ca-certificates，只能用 http，装完即弃不影响运行时安全）
RUN sed -i 's|http://archive.ubuntu.com|http://mirrors.aliyun.com|g; s|http://security.ubuntu.com|http://mirrors.aliyun.com|g' /etc/apt/sources.list \
    && apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libpcap0.8 \
    iptables \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 从构建阶段复制二进制文件（前端已嵌入）
COPY --from=builder /app/netbouncer .

# 暴露端口
EXPOSE 8080

# 运行应用
CMD ["./netbouncer"]
