#!/usr/bin/env bash
# 本地命令入口（Windows Git Bash / macOS / Linux 通用）。
#
# 用法：
#   bash scripts/local.sh <command>
#
# 命令：
#   dev           启动后端（托管 frontend/build/web，端口 8080）
#   build         编译后端二进制到 dist/
#   web           构建 Flutter 产物
#   web-build     构建 Vue 前端（Vite → frontend/dist）
#   web-dev       启动 Vite 开发服务器（/api 代理到 127.0.0.1:8080）
#   web-test      Vue 单元测试（Vitest）
#   web-typecheck Vue 类型检查（vue-tsc）
#   web-compat    扫描构建产物 es2020 语法兼容性 + §14.7 API/CSS 特性（es-check + compat.mjs）
#   web-legacy    重建 Flutter 旧版到 frontend/dist/legacy 并校验
#   test          后端 vet + 全部测试（严格 bean-check）
#   test-web      前端静态分析 + 单元测试
#   check         test + 版本校验 + bean-check 自检（CI 同款）
#   version       版本号三处一致性校验
#   smoke         端到端冒烟
#   rehearse      迁移 012 副本库预演（VACUUM INTO 快照 + 两次迁移 + bean-check）
#
# 存在原因：本机 Windows 无 make，Makefile 无法直接使用。

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

# Go 与 Flutter 可能不在 PATH（AGENTS.md 记录的本机路径作为兜底）
if ! command -v go >/dev/null 2>&1 && [[ -x /d/Environment/go/bin/go ]]; then
  export PATH="$PATH:/d/Environment/go/bin"
fi
if ! command -v flutter >/dev/null 2>&1 && [[ -x /d/Environment/flutter/bin/flutter ]]; then
  export PATH="$PATH:/d/Environment/flutter/bin"
fi

# Flutter 的 flutter_tester 通过 127.0.0.1 上的 WebSocket 与测试驱动通信。
# 若环境设了全局 HTTP 代理却没放行回环地址，代理会劫持这个本地连接，
# 表现为所有测试文件在「加载阶段」报：
#   Unable to connect to flutter_tester process: WebSocketException: Invalid WebSocket upgrade request
# 这里始终放行本地回环（保留已有 NO_PROXY 值），不影响外网依赖下载。
export NO_PROXY="${NO_PROXY:+$NO_PROXY,}localhost,127.0.0.1,::1"
export no_proxy="${no_proxy:+$no_proxy,}localhost,127.0.0.1,::1"

cmd="${1:-help}"

case "$cmd" in
  dev)
    echo "启动后端（web=frontend/build/web）…"
    LEDGER_WEB_ROOT="${LEDGER_WEB_ROOT:-frontend/build/web}" go run ./cmd/server
    ;;

  build)
    mkdir -p dist
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dist/server ./cmd/server
    echo "✅ dist/server"
    ;;

  web)
    ( cd frontend && flutter build web --release )
    echo "✅ frontend/build/web"
    ;;

  web-build)
    ( cd frontend && npm run build )
    echo "✅ frontend/dist"
    ;;

  web-dev)
    ( cd frontend && npm run dev )
    ;;

  web-test)
    ( cd frontend && npm run test )
    ;;

  web-typecheck)
    ( cd frontend && npm run typecheck )
    ;;

  web-compat)
    ( cd frontend && npm run compat )
    ;;

  web-legacy)
    bash scripts/build-legacy.sh
    ;;

  test)
    go vet ./...
    BEAN_CHECK_STRICT=1 go test ./...
    ;;

  test-web)
    ( cd frontend && flutter analyze lib && flutter test )
    ;;

  version)
    bash scripts/verify-version.sh
    ;;

  check)
    go vet ./...
    BEAN_CHECK_STRICT=1 go test ./...
    bash scripts/verify-version.sh
    bash scripts/check-bean.sh
    echo ""
    echo "✅ check 全部通过"
    ;;

  smoke)
    bash scripts/smoke.sh
    ;;

  rehearse)
    bash scripts/rehearsal.sh
    ;;

  help|*)
    sed -n '2,24p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
    ;;
esac
