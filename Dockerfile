# 第一阶段：前端构建
FROM node:alpine AS frontend-builder

ARG VERSION=1.0.0
WORKDIR /build/web

# 先复制依赖清单，利用 Docker 缓存
COPY web/package*.json ./
RUN npm ci

# 再复制源码并构建
COPY web/ ./
RUN VITE_VERSION=${VERSION} npm run build


# 第二阶段：Go 后端构建
FROM golang:alpine AS go-builder

ARG VERSION=1.0.0
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /build

# 先复制 Go 依赖清单，利用 Docker 缓存（最关键！）
COPY go.mod go.sum ./
RUN go mod download

# 复制前端构建产物
COPY --from=frontend-builder /build/web/dist ./web/dist

# 复制 Go 源码（依赖之外的文件）
COPY . .

# 编译二进制文件
RUN go build -ldflags "-s -w -X gpt-load/internal/version.Version=${VERSION}" -o gpt-load


FROM alpine

WORKDIR /app
RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates tzdata \
    && update-ca-certificates

COPY --from=go-builder /build/gpt-load .
EXPOSE 3001
ENTRYPOINT ["/app/gpt-load"]
