# FamBean 单镜像构建：Vue Web 产物（Node/Vite）+ Go 静态二进制 + 最小运行时。
#
# 设计要点：
#   - 三阶段构建，Node 构建环境只存在于 builder 阶段，不进最终镜像
#   - Node 版本与 frontend/.nvmrc、CI web-verify job 三处一致（§14.7 钉版）
#   - Go 用 CGO_ENABLED=0 静态编译（modernc.org/sqlite 纯 Go，无需 C 工具链）
#   - 运行时用 alpine + 非 root 用户，仅含二进制、前端产物与 schema
#   - 单容器单端口：Go 进程同时提供 API 与前端静态资源（见 internal/server/static.go）
#
# 历史：批 6 cutover 前为 Flutter 三阶段构建（ghcr.io/cirruslabs/flutter:3.35.1）；
#   Dart 源码清退后改为 Node 构建，镜像体积与构建时长显著下降。
#
# 构建：
#   docker build -t famben:dev .
#   docker build --build-arg GIT_VERSION=0.5.0 --build-arg BUILD_HASH=$(git rev-parse --short HEAD) -t famben:0.5.0 .
#
# 运行：
#   docker run -d -p 8080:8080 -v $(pwd)/data:/app/data famben:dev

# syntax=docker/dockerfile:1

# 镜像版本集中在此，便于升级时只改一处。
# Node 版本策略：与 frontend/.nvmrc（22.23.4）及 package.json engines（>=22.22 <23）对齐；
# 升级须同步改 .nvmrc、engines、CI setup-node 三处（verify-version / web-verify 均会卡不一致）。
ARG NODE_IMAGE=node:22-alpine
ARG GO_IMAGE=golang:1.25-alpine
ARG RUNTIME_IMAGE=alpine:3.20

# ---------- Stage 1：构建 Vue Web ----------
FROM ${NODE_IMAGE} AS web-builder

WORKDIR /src/frontend

# 先只拷依赖清单，让 npm ci 层可缓存（源码变动不必重装依赖）。
# package-lock.json 含全平台 optional 二进制（@esbuild/*、@rollup/*），
# 在 linux/alpine 内 npm ci 可直接命中平台包，无需重新解析。
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build
# 产物在 /src/frontend/dist（Vite outDir），由 Stage 3 拷贝。

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
# su-exec：入口脚本降权用（比 gosu 轻量，alpine 官方仓库有）
RUN apk add --no-cache ca-certificates tzdata su-exec \
    && adduser -D -u 10001 -h /app app

WORKDIR /app

COPY --from=go-builder /out/server /app/server
COPY --from=web-builder /src/frontend/dist /app/web
COPY schema.sql /app/schema.sql
COPY docker-entrypoint.sh /entrypoint.sh

# 数据目录（挂载点）：数据库与 backups/ 都落在这里
RUN mkdir -p /app/data \
    && chmod +x /entrypoint.sh \
    && chown -R app:app /app

# 这里刻意不写 USER：
#   容器以 root 启动，唯一目的是让入口脚本把 bind mount 过来的数据目录
#   属主修正为 10001（否则 SQLite 建 -wal/-shm 会失败，报 error 14）。
#   脚本随后立即用 su-exec 降权，最终的服务进程（PID 1）仍是非 root 的 10001。
#   如需「全程非 root」，在 compose 里指定 user: "10001:10001" 即可，
#   此时入口脚本检测到非 root 会跳过修正直接 exec。

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

ENTRYPOINT ["/entrypoint.sh"]
CMD ["/app/server"]
