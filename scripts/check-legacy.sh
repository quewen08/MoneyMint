#!/usr/bin/env bash
# 校验旧版产物确实以 /legacy/ 为基路径（S-A02 回滚前提）。
#
# 断言：
#   1. frontend/dist/legacy/index.html 存在；
#   2. 含 <base href="/legacy/">（或等价 href="/legacy/"）；
#   3. 不含 href="/"（否则说明产物仍是根路径构建，挪目录后会白屏）。
#
# 用法：bash scripts/check-legacy.sh

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INDEX="$ROOT_DIR/frontend/dist/legacy/index.html"

if [[ ! -f "$INDEX" ]]; then
  echo "❌ 未找到 $INDEX"
  echo "   请先执行 bash scripts/build-legacy.sh"
  exit 1
fi

if ! grep -q 'href="/legacy/"' "$INDEX"; then
  echo "❌ $INDEX 未包含 href=\"/legacy/\"，旧版资源会打到根路径"
  exit 1
fi

if grep -q 'href="/"' "$INDEX"; then
  echo "❌ $INDEX 仍包含 href=\"/\"（根路径构建产物，不能用于 /legacy/ 回滚）"
  exit 1
fi

echo "✅ legacy 基路径校验通过：$INDEX"
