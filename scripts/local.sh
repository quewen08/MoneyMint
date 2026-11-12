#!/usr/bin/env bash
# 本地命令入口（Windows Git Bash / macOS / Linux 通用）。
#
# 用法：
#   bash scripts/local.sh <command>
#
# 命令：
#   dev           启动后端（托管 frontend/dist，端口 8080）
#   build         编译后端二进制到 dist/
#   web-build     构建 Vue 前端（Vite → frontend/dist）
#   web-dev       启动 Vite 开发服务器（/api 代理到 127.0.0.1:8080）
#   web-test      Vue 单元测试（Vitest）
#   web-typecheck Vue 类型检查（vue-tsc）
#   web-compat    扫描构建产物 es2020 语法兼容性 + §14.7 API/CSS 特性（es-check + compat.mjs）
#   check-legacy  校验冻结的 /legacy/ 回滚基准产物（dist/legacy/，不再重建）
#   test          后端 vet + 全部测试（严格 bean-check）
#   test-web      前端 Vitest（与 web-test 同义，历史命令名保留）
#   check         test + 版本校验 + bean-check 自检（CI 同款）
#   version       版本号三处一致性校验
#   smoke         端到端冒烟
#   rehearse      迁移 012 副本库预演（VACUUM INTO 快照 + 两次迁移 + bean-check）
#
# cutover（批 6）后仓库只有一套前端；Flutter 源码与构建链已清退（git 历史可查）。
# dist/legacy/ 为冻结的旧版回滚基准，只读不改，损坏时从 git 历史（batch ≤5 tag/commit）
# 找回 frontend/dist/legacy 或旧源码重建。
#
# 存在原因：本机 Windows 无 make，Makefile 无法直接使用。

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

# Go 可能不在 PATH（AGENTS.md 记录的本机路径作为兜底）
if ! command -v go >/dev/null 2>&1 && [[ -x /d/Environment/go/bin/go ]]; then
  export PATH="$PATH:/d/Environment/go/bin"
fi

# 始终放行本地回环（保留已有 NO_PROXY 值），避免全局代理劫持 127.0.0.1 的
# Vite→后端 /api 代理与本地冒烟请求。
export NO_PROXY="${NO_PROXY:+$NO_PROXY,}localhost,127.0.0.1,::1"
export no_proxy="${no_proxy:+$no_proxy,}localhost,127.0.0.1,::1"

cmd="${1:-help}"

case "$cmd" in
  dev)
    echo "启动后端（web=frontend/dist）…"
    LEDGER_WEB_ROOT="${LEDGER_WEB_ROOT:-frontend/dist}" go run ./cmd/server
    ;;

  build)
    mkdir -p dist
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dist/server ./cmd/server
    echo "✅ dist/server"
    ;;

  web-build)
    ( cd frontend && npm run build )
    # 注意：emptyOutDir 会清空 dist/，若本地还有冻结的 dist/legacy/ 会一并被清掉。
    # cutover 后 legacy 仅作回滚基准，需要时从 git 历史找回（见文件头说明）。
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

  check-legacy)
    bash scripts/check-legacy.sh
    ;;

  test)
    go vet ./...
    BEAN_CHECK_STRICT=1 go test ./...
    ;;

  test-web)
    ( cd frontend && npm run test )
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
    sed -n '2,28p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
    ;;
esac
