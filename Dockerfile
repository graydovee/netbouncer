# Node.js构建阶段 - 构建React前端
FROM node:18-alpine AS frontend-builder

WORKDIR /app/frontend

# 复制package文件
COPY website/package*.json ./

# 安装所有依赖（包括开发依赖）
RUN npm ci

# 复制前端源代码
COPY website/ .

# 构建前端项目
RUN npm run build

# Go构建阶段
FROM golang:1.24.3-bullseye AS builder

# 安装依赖
RUN apt-get update && apt-get install -y \
    libpcap-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 复制go mod文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
ARG TARGETPLATFORM
ARG BUILDPLATFORM
RUN CGO_ENABLED=1 GOOS=linux go build -o netbouncer main.go

# 运行阶段
FROM ubuntu:22.04

# 安装运行时依赖
RUN apt-get update && apt-get install -y --no-install-recommends \
    libpcap0.8 \
    iptables \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/netbouncer .
# 从frontend-builder复制构建好的前端文件
COPY --from=frontend-builder /app/frontend/dist ./web

# 设置环境变量
ENV GIN_MODE=release

# 暴露端口
EXPOSE 8080

# 运行应用
CMD ["./netbouncer"]
