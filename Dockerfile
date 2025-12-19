# 多阶段构建：前端构建阶段
FROM node:20-alpine as frontend-builder

# 设置工作目录
WORKDIR /app/frontend

# 复制前端依赖文件
COPY frontend/package.json frontend/pnpm-lock.yaml ./

# 安装依赖
RUN npm install pnpm -g && pnpm install --frozen-lockfile

# 复制前端代码
COPY frontend/ .

# 构建前端
RUN pnpm run build

# 后端构建阶段
FROM python:3.11-alpine as backend-builder

# 设置工作目录
WORKDIR /app/backend

# 复制后端依赖文件
COPY backend/ ./

# 安装后端依赖
RUN apk add --no-cache gcc musl-dev bison flex meson && python -m venv .venv && source .venv/bin/activate && pip install --no-cache-dir -r requirements.txt

# 最终运行阶段
FROM python:3.11-alpine

# 按照nodejs的alpine镜像安装nodejs 废弃，太耗时，使用apk安装
# ENV NODE_VERSION=20.19.6

# RUN addgroup -g 1000 node \
#     && adduser -u 1000 -G node -s /bin/sh -D node \
#     && apk add --no-cache \
#         libstdc++ \
#     && apk add --no-cache --virtual .build-deps \
#         curl \
#     && ARCH= OPENSSL_ARCH='linux*' && alpineArch="$(apk --print-arch)" \
#       && case "${alpineArch##*-}" in \
#         x86_64) ARCH='x64' CHECKSUM="a371d92fafee1b20ede35c3df747ca1c8b25fcb2e14d3a4c36b41166faae707f" OPENSSL_ARCH=linux-x86_64;; \
#         x86) OPENSSL_ARCH=linux-elf;; \
#         aarch64) OPENSSL_ARCH=linux-aarch64;; \
#         arm*) OPENSSL_ARCH=linux-armv4;; \
#         ppc64le) OPENSSL_ARCH=linux-ppc64le;; \
#         s390x) OPENSSL_ARCH=linux-s390x;; \
#         *) ;; \
#       esac \
#   && if [ -n "${CHECKSUM}" ]; then \
#     set -eu; \
#     curl -fsSLO --compressed "https://unofficial-builds.nodejs.org/download/release/v$NODE_VERSION/node-v$NODE_VERSION-linux-$ARCH-musl.tar.xz"; \
#     echo "$CHECKSUM  node-v$NODE_VERSION-linux-$ARCH-musl.tar.xz" | sha256sum -c - \
#       && tar -xJf "node-v$NODE_VERSION-linux-$ARCH-musl.tar.xz" -C /usr/local --strip-components=1 --no-same-owner \
#       && ln -s /usr/local/bin/node /usr/local/bin/nodejs; \
#   else \
#     echo "Building from source" \
#     # backup build
#     && apk add --no-cache --virtual .build-deps-full \
#         binutils-gold \
#         g++ \
#         gcc \
#         gnupg \
#         libgcc \
#         linux-headers \
#         make \
#         python3 \
#         py-setuptools \
#     # use pre-existing gpg directory, see https://github.com/nodejs/docker-node/pull/1895#issuecomment-1550389150
#     && export GNUPGHOME="$(mktemp -d)" \
#     # gpg keys listed at https://github.com/nodejs/node#release-keys
#     && for key in \
#       5BE8A3F6C8A5C01D106C0AD820B1A390B168D356 \
#       DD792F5973C6DE52C432CBDAC77ABFA00DDBF2B7 \
#       CC68F5A3106FF448322E48ED27F5E38D5B0A215F \
#       8FCCA13FEF1D0C2E91008E09770F7A9A5AE15600 \
#       890C08DB8579162FEE0DF9DB8BEAB4DFCF555EF4 \
#       C82FA3AE1CBEDC6BE46B9360C43CEC45C17AB93C \
#       108F52B48DB57BB0CC439B2997B01419BD92F80A \
#       A363A499291CBBC940DD62E41F10027AF002F8B0 \
#     ; do \
#       { gpg --batch --keyserver hkps://keys.openpgp.org --recv-keys "$key" && gpg --batch --fingerprint "$key"; } || \
#       { gpg --batch --keyserver keyserver.ubuntu.com --recv-keys "$key" && gpg --batch --fingerprint "$key"; } ; \
#     done \
#     && curl -fsSLO --compressed "https://nodejs.org/dist/v$NODE_VERSION/node-v$NODE_VERSION.tar.xz" \
#     && curl -fsSLO --compressed "https://nodejs.org/dist/v$NODE_VERSION/SHASUMS256.txt.asc" \
#     && gpg --batch --decrypt --output SHASUMS256.txt SHASUMS256.txt.asc \
#     && gpgconf --kill all \
#     && rm -rf "$GNUPGHOME" \
#     && grep " node-v$NODE_VERSION.tar.xz\$" SHASUMS256.txt | sha256sum -c - \
#     && tar -xf "node-v$NODE_VERSION.tar.xz" \
#     && cd "node-v$NODE_VERSION" \
#     && ./configure \
#     && make -j$(getconf _NPROCESSORS_ONLN) V= \
#     && make install \
#     && apk del .build-deps-full \
#     && cd .. \
#     && rm -Rf "node-v$NODE_VERSION" \
#     && rm "node-v$NODE_VERSION.tar.xz" SHASUMS256.txt.asc SHASUMS256.txt; \
#   fi \
#   && rm -f "node-v$NODE_VERSION-linux-$ARCH-musl.tar.xz" \
#   # Remove unused OpenSSL headers to save ~34MB. See this NodeJS issue: https://github.com/nodejs/node/issues/46451
#   && find /usr/local/include/node/openssl/archs -mindepth 1 -maxdepth 1 ! -name "$OPENSSL_ARCH" -exec rm -rf {} \; \
#   && apk del .build-deps \
#   # smoke tests
#   && node --version \
#   && npm --version \
#   && rm -rf /tmp/*

# 安装nodejs
RUN apk add --no-cache nodejs npm

# 设置工作目录
WORKDIR /app

# 安装PM2
RUN npm install pm2 -g

# 复制后端代码和依赖
COPY --from=backend-builder /app/backend ./backend
# 复制虚拟环境
COPY --from=backend-builder /app/backend/.venv ./backend/.venv

# 复制前端构建产物
COPY --from=frontend-builder /app/frontend/.output ./frontend/.output
COPY --from=frontend-builder /app/frontend/ecosystem.config.cjs ./frontend/

# 创建数据目录
RUN mkdir -p ./backend/data

# 创建环境变量文件
RUN echo "LEDGER_FILE=/app/backend/data/main.bean" > ./backend/.env

# 提取版本号并设置为环境变量
RUN VERSION=$(cat /app/backend/app/version.py | grep -E '^__version__' | cut -d"'" -f2) && echo "APP_VERSION=$VERSION" > /app/.env

# 启动脚本（使用ash兼容语法）
RUN printf '#!/bin/sh \n
cd /app/backend && . /app/backend/.venv/bin/activate && python run.py &\n
cd /app/frontend && pm2-runtime ecosystem.config.cjs' > /app/start.sh && chmod +x /app/start.sh

# 检查启动脚本是否存在并具有执行权限
RUN ls -la /app/start.sh && cat /app/start.sh

# 暴露端口
EXPOSE 3000

# 启动应用（使用ash shell）
CMD ["/bin/sh", "/app/start.sh"]