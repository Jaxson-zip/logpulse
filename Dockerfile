# ---- 构建阶段:基于 golang:alpine,编译静态二进制 ----
FROM golang:1.25-alpine AS builder

WORKDIR /src

# 先复制依赖清单,利用层缓存加速重建
COPY go.mod go.sum ./
RUN go mod download

# 复制源码并构建:禁用 CGO 生成纯静态二进制,去掉调试信息缩减体积
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/logpulse \
    .

# ---- 运行阶段:最小 alpine 镜像 ----
FROM alpine:3.21

# 安装 ca-certificates 以支持后续可能的 HTTPS 抓取;非 root 用户运行
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S app -G app

COPY --from=builder /out/logpulse /usr/local/bin/logpulse

USER app
ENTRYPOINT ["logpulse"]
CMD ["--help"]
