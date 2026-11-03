# FamBean 单镜像构建：Flutter Web 产物 + Go 静态二进制 + 最小运行时。
#
# 设计要点：
#   - 三阶段构建，Flutter SDK（约 2GB）只存在于 builder 阶段，不进最终镜像
#   - Go 用 CGO_ENABLED=0 静态编译（modernc.org/sqlite 纯 Go，无需 C 工具链）
#   - 运行时用 alpine + 非 root 用户，仅含二进制、前端产物与 schema
#   - 单容器单端口：Go 进程同时提供 API 与前端静态资源（见 internal/server/static.go）
#
# 构建：
#   docker build -t famben:dev .
#   docker build --build-arg GIT_VERSION=0.5.0 --build-arg BUILD_HASH=$(git rev-parse --short HEAD) -t famben:0.5.0 .
#
# 运行：
#   docker run -d -p 8080:8080 -v $(pwd)/data:/app/data famben:dev

# syntax=docker/dockerfile:1

# 镜像版本集中在此，便于升级时只改一处
ARG FLUTTER_IMAGE=ghcr.io/cirruslabs/flutter:3.35.1
ARG GO_IMAGE=golang:1.25-alpine
ARG RUNTIME_IMAGE=alpine:3.20

# ---------- Stage 1：构建 Flutter Web ----------
FROM ${FLUTTER_IMAGE} AS web-builder

WORKDIR /src/frontend

# 先只拷依赖清单，让 pub get 层可缓存（源码变动不必重装依赖）
COPY frontend/pubspec.yaml frontend/pubspec.lock ./
# --enforce-lockfile：pubspec.lock 与 pubspec.yaml 不一致时构建失败，
# 避免「本地能跑、镜像里依赖漂移」的隐蔽问题。
RUN flutter pub get --enforce-lockfile

COPY frontend/ ./
# 说明：本项目以 emoji 作账户图标，不走 Material Icons 字体子集化，
# 故不启用 --tree-shake-icons（启用后反而可能误删图标字体）。
RUN flutter build web --release

# ---------- Stage 2：构建 Go 二进制 ----------
FROM ${GO_IMAGE} AS go-builder

WORKDIR /src

# 依赖层缓存
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG BUILD_DATE=unknown
ARG BUILD_HASH=unknown
ARG GIT_VERSION=unknown

# -trimpath 去掉本机路径（构建可复现）；-s -w 去掉符号表与调试信息（减体积）
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
      -ldflags "-s -w \
        -X main.buildDate=${BUILD_DATE} \
        -X main.buildHash=${BUILD_HASH} \
        -X main.gitVersion=${GIT_VERSION}" \
      -o /out/server ./cmd/server

# ---------- Stage 3：运行时 ----------
FROM ${RUNTIME_IMAGE}

# tzdata：交易日与「每月起始日」依赖本地时区；ca-certificates：未来外部调用需要
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 -h /app app

WORKDIR /app

COPY --from=go-builder /out/server /app/server
COPY --from=web-builder /src/frontend/build/web /app/web
COPY schema.sql /app/schema.sql

# 数据目录（挂载点）：数据库与 backups/ 都落在这里
RUN mkdir -p /app/data && chown -R app:app /app

# 非 root 运行：容器逃逸后的影响面最小化
USER app

ENV LEDGER_DB=/app/data/ledger.db \
    LEDGER_SCHEMA=/app/schema.sql \
    LEDGER_WEB_ROOT=/app/web \
    LISTEN=:8080 \
    TZ=Asia/Shanghai

EXPOSE 8080
VOLUME ["/app/data"]

# 健康检查：容器编排与 NAS 面板可据此判断存活
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -O- http://127.0.0.1:8080/api/health >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/app/server"]
