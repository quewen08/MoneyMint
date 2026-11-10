#!/usr/bin/env bash
# 重建 Flutter 旧版产物并复制到 frontend/dist/legacy（S-A02）。
#
# 为什么必须重建：现成产物 base href 写死为 "/"，挪到 /legacy/ 后资源会打到根路径，
# 被 Vue 接管而白屏。用 --base-href /legacy/ 重建后再复制，旧版主流程保持可用。
#
# 用法：bash scripts/build-legacy.sh
# 注意：`npm run build` 的 emptyOutDir 会清空 frontend/dist，
#       因此每次 web:build 之后都需要重新跑一次本脚本（或在部署前跑）。

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

# Flutter 可能不在 PATH（AGENTS.md 记录的本机路径作为兜底）
if ! command -v flutter >/dev/null 2>&1 && [[ -x /d/Environment/flutter/bin/flutter ]]; then
  export PATH="$PATH:/d/Environment/flutter/bin"
fi
export PUB_HOSTED_URL="${PUB_HOSTED_URL:-https://pub.flutter-io.cn}"
# Git Bash（MSYS）会把参数里的 /legacy/ 当成本机路径转换成 C:/Program Files/Git/legacy/，
# 导致 Flutter 报「--base-href should start and end with /」。关闭参数路径转换。
export MSYS_NO_PATHCONV=1
export MSYS2_ARG_CONV_EXCL='*'

OUT_DIR="$ROOT_DIR/frontend/build/legacy-web"
# Flutter 的 shader 编译（impellerc）在 Windows 下不接受 MSYS 的 /d/... 路径，
# 会拼出 /d/...\assets 这种混合分隔符而写盘失败；用 cygpath -w 转成 D:\... 原生路径。
if command -v cygpath >/dev/null 2>&1; then
  OUT_ARG="$(cygpath -w "$OUT_DIR")"
else
  OUT_ARG="$OUT_DIR"
fi

echo "构建 Flutter 旧版（base-href=/legacy/）…"
( cd frontend && flutter build web --release --base-href /legacy/ --output "$OUT_ARG" )

node scripts/copy-legacy.mjs
bash scripts/check-legacy.sh
