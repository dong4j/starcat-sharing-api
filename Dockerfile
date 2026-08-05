# ===========================================
# Stage 1: 构建阶段
# ===========================================
# 与 starcat-trending-api / starcat-weekly-api 保持一致的多阶段构建
# Go 版本由项目 go.mod 决定 (1.25.0)
FROM golang:1.25-alpine AS builder

ARG VERSION=0.0.0-dev

# 设置工作目录
WORKDIR /app

# 先复制依赖文件, 利用 Docker 缓存
# 先复制 go.mod / go.sum，避免业务源码变化导致每次都重新下载依赖。
COPY go.mod go.sum* ./
RUN go mod download

# 复制源码并编译
COPY . .
# CGO_ENABLED=0 编译为静态二进制, 适用于 alpine / scratch
# VERSION 由发布 tag 注入；默认值只用于本地开发构建。
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X github.com/starcat-app/starcat-sharing-api/internal/version.Version=${VERSION}" \
    -o /app/bin/server \
    ./cmd/server/

# ===========================================
# Stage 2: 运行阶段
# ===========================================
# 使用 alpine 基础镜像保持体积小巧
FROM alpine:3.20

# 安装 CA 证书 + tzdata (后者对时间格式化/日志有用)
RUN apk --no-cache add ca-certificates tzdata

# 设置时区
ENV TZ=UTC

# 创建非 root 用户运行服务 (安全最佳实践)
RUN addgroup -S app && adduser -S app -G app

# 工作目录固定在 /app
# - /app/server   : 编译产物
# - /app/templates: HTML 模板 (main.go 启动时 template.ParseGlob 读取)
# 数据文件路径由 STORE_FILE 环境变量指向挂载卷 /data, 不在镜像中
WORKDIR /app

# 从 builder 阶段复制编译产物和模板
COPY --from=builder /app/bin/server /app/server
COPY --from=builder /app/templates /app/templates
COPY --from=builder /app/static /app/static

# 切换到非 root 用户
USER app

# 暴露端口 (与 main.go 默认 5001 一致)
EXPOSE 5001

# 健康检查 (与 main.go /healthz 端点对齐: 30s/5s/10s grace)
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:5001/healthz || exit 1

# 启动服务
ENTRYPOINT ["/app/server"]
