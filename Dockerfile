# Node.js构建阶段 - 构建React前端
# 前端产物是静态文件，与运行架构无关：固定在本机构建平台跑一次，两架构共用
FROM --platform=$BUILDPLATFORM node:18-alpine AS frontend-builder

WORKDIR /app/frontend

# 复制package文件
COPY website/package*.json ./

# 安装所有依赖（包括开发依赖；npm 走国内镜像，proxy.golang.org/registry.npmjs.org 直连不稳定）
RUN npm config set registry https://registry.npmmirror.com && npm ci

# 复制前端源代码
COPY website/ .

# 构建前端项目
RUN npm run build

# Go构建阶段 - 固定在本机构建平台，用目标架构工具链交叉编译（CGO），避免 QEMU 模拟
FROM --platform=$BUILDPLATFORM golang:1.25.7-bookworm AS builder

ARG TARGETOS
ARG TARGETARCH

# 交叉编译工具链（CGO：sqlite/pcap 需要目标架构的 C 工具链与 libpcap）：
# - 与构建平台同架构的目标直接用原生 gcc，无需交叉包；
# - 异架构目标用 Debian 的交叉工具链（只存在部分宿主组合，显式限定宿主架构）
# - libpcap0.8-dev 两个目标架构都装（Multi-Arch: same，头文件可共存）
# - apt 走国内镜像，构建环境在国内网络时 deb.debian.org 直连极慢
RUN set -eux; \
    sed -i 's|http://deb.debian.org|https://mirrors.aliyun.com|g' /etc/apt/sources.list.d/debian.sources; \
    HOSTARCH="$(dpkg --print-architecture)"; \
    case "$HOSTARCH" in \
      arm64) CROSSARCH=amd64; CROSSPAK=x86-64; CROSSGCC=x86_64-linux-gnu-gcc; CROSSDIR=/usr/x86_64-linux-gnu ;; \
      amd64) CROSSARCH=arm64; CROSSPAK=aarch64; CROSSGCC=aarch64-linux-gnu-gcc; CROSSDIR=/usr/aarch64-linux-gnu ;; \
      *) echo "unsupported builder arch: $HOSTARCH"; exit 1 ;; \
    esac; \
    dpkg --add-architecture arm64; dpkg --add-architecture amd64; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      libpcap0.8-dev:arm64 \
      libpcap0.8-dev:amd64 \
      "gcc-$CROSSPAK-linux-gnu:$HOSTARCH" \
      "libc6-dev-$CROSSARCH-cross:$HOSTARCH"; \
    rm -rf /var/lib/apt/lists/*; \
    # 把 pcap 头文件与链接库放入交叉工具链的搜索路径
    # （不能直接 -I/usr/include，会用宿主架构的 libc 头覆盖交叉 sysroot）
    cp /usr/include/pcap*.h "$CROSSDIR/include/"; \
    cp -r /usr/include/pcap "$CROSSDIR/include/"; \
    ln -s "/usr/lib/$("$CROSSGCC" -dumpmachine)/libpcap.so" "$CROSSDIR/lib/libpcap.so"

WORKDIR /app

# Go 模块代理走国内（proxy.golang.org 直连不稳定）
ENV GOPROXY=https://goproxy.cn,direct

# 复制go mod文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 复制前端构建产物，嵌入二进制实现单文件部署
COPY --from=frontend-builder /app/frontend/dist ./pkg/web/dist

# 按目标架构编译：同架构用原生 gcc，异架构用上面装好的交叉工具链
RUN set -eux; \
    HOSTARCH="$(dpkg --print-architecture)"; \
    if [ "$TARGETARCH" = "$HOSTARCH" ]; then \
      CC=; \
    else \
      case "$TARGETARCH" in \
        amd64) CC=x86_64-linux-gnu-gcc ;; \
        arm64) CC=aarch64-linux-gnu-gcc ;; \
        *) echo "unsupported target arch: $TARGETARCH"; exit 1 ;; \
      esac; \
    fi; \
    export CC; \
    CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -tags embed -o netbouncer main.go

# 运行阶段
FROM ubuntu:22.04

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
