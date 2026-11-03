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

# 镜像版本集中在此，便于升级时只改一处。
#
# 注意 Flutter 版本差异：
#   此处 3.35.1 已验证可拉取；但开发机本地 Flutter 可能是别的版本（当前为 3.38.9）。
#   两者不一致会导致 pubspec.lock 在镜像内被重新解析（下方 pub get 步骤会告警）。
#   若要彻底消除差异，把 FLUTTER_IMAGE 改成开发机 `flutter --version` 的版本，
#   并确认 ghcr.io/cirruslabs/flutter 存在对应 tag。
ARG FLUTTER_IMAGE=ghcr.io/cirruslabs/flutter:3.35.1
ARG GO_IMAGE=golang:1.25-alpine
ARG RUNTIME_IMAGE=alpine:3.20

# ---------- Stage 1：构建 Flutter Web ----------
FROM ${FLUTTER_IMAGE} AS web-builder

WORKDIR /src/frontend

# 先只拷依赖清单，让 pub get 层可缓存（源码变动不必重装依赖）
COPY frontend/pubspec.yaml frontend/pubspec.lock ./

# 不使用 --enforce-lockfile（曾被它卡死过一次构建，exit 65）：
#   pubspec.lock 是开发机 Flutter SDK 生成的，其中 meta / vm_service / test_api / crypto
#   属于「随 SDK 捆绑」的包，版本由 SDK 决定而非 pub 解析。镜像内 SDK 与生成 lockfile
#   的 SDK 版本不同时，pub 必然要求改动这些包，--enforce-lockfile 会直接判失败。
# 改成「正常解析 + 漂移告警」：不因 SDK 微差阻断整个镜像构建，但绝不静默——
#   一旦 lockfile 真的被重新解析，构建日志会明确打出来，便于判断是否需要对齐版本。
RUN cp pubspec.lock /tmp/pubspec.lock.orig \
    && flutter pub get \
    && if ! cmp -s /tmp/pubspec.lock.orig pubspec.lock; then \
         echo "⚠️  警告：pubspec.lock 在镜像内被重新解析了"; \
         echo "    原因：镜像的 Flutter SDK 与生成该 lockfile 的版本不同（SDK 捆绑包 meta/vm_service/test_api 等被调整）。"; \
         echo "    影响：本次构建使用镜像内解析结果，可能与开发机依赖版本有细微差异。"; \
         echo "    如需完全复现：把上方 FLUTTER_IMAGE 钉到与开发机一致的 Flutter 版本。"; \
       fi

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
