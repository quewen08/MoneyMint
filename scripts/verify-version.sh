#!/usr/bin/env bash
# 版本号一致性校验：git tag / CHANGELOG.md / frontend/pubspec.yaml 三处必须一致。
#
# 用途：
#   - CI 发布前校验（传 tag 参数，最严格）
#   - 本地随手校验（不传参数，tag 取 git describe；无 tag 时只比对 CHANGELOG 与 pubspec）
#
# 退出码：0 = 一致；1 = 存在漂移
#
# 为什么以 git tag 为单一事实源：
#   之前 pubspec.yaml 写 1.0.0、CHANGELOG 写 0.4.0，两处长期漂移却无人发现。
#   镜像 tag 必须来自一个可信来源，tag 不可被顺手改动，最适合作为锚点。

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

CHANGELOG_FILE="CHANGELOG.md"
PUBSPEC_FILE="frontend/pubspec.yaml"

# ---- 1) 取 git tag ----
TAG="${1:-}"
if [[ -z "$TAG" ]]; then
  TAG="$(git describe --tags --abbrev=0 2>/dev/null || true)"
fi
TAG_VER=""
if [[ -n "$TAG" ]]; then
  TAG_VER="${TAG#v}"   # 允许 v0.5.0 或 0.5.0 两种写法
fi

# ---- 2) 取 CHANGELOG 顶部版本（首个 ## [x.y.z]）----
CHANGELOG_VER="$(grep -oE '^## \[[0-9]+\.[0-9]+\.[0-9]+\]' "$CHANGELOG_FILE" 2>/dev/null \
  | head -1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || true)"

# ---- 3) 取 pubspec 版本（去掉 +build 后缀）----
PUBSPEC_VER="$(grep -E '^version:' "$PUBSPEC_FILE" 2>/dev/null \
  | head -1 \
  | sed -E 's/^version:[[:space:]]*([0-9]+\.[0-9]+\.[0-9]+).*/\1/' || true)"

echo "版本号校验"
echo "  git tag    = ${TAG_VER:-（无 tag）}"
echo "  CHANGELOG  = ${CHANGELOG_VER:-（未找到）}"
echo "  pubspec    = ${PUBSPEC_VER:-（未找到）}"

FAIL=0

if [[ -z "$CHANGELOG_VER" ]]; then
  echo "  ❌ 无法从 $CHANGELOG_FILE 解析出版本号（应形如 '## [0.5.0] - 2026-11-03'）" >&2
  FAIL=1
fi
if [[ -z "$PUBSPEC_VER" ]]; then
  echo "  ❌ 无法从 $PUBSPEC_FILE 解析出版本号（应形如 'version: 0.5.0+1'）" >&2
  FAIL=1
fi

if [[ $FAIL -eq 0 && "$CHANGELOG_VER" != "$PUBSPEC_VER" ]]; then
  echo "  ❌ CHANGELOG($CHANGELOG_VER) 与 pubspec($PUBSPEC_VER) 不一致" >&2
  FAIL=1
fi

if [[ $FAIL -eq 0 && -n "$TAG_VER" && "$TAG_VER" != "$CHANGELOG_VER" ]]; then
  echo "  ❌ git tag($TAG_VER) 与 CHANGELOG($CHANGELOG_VER) 不一致" >&2
  echo "     发布新版本时请同时更新 CHANGELOG.md 顶部版本与 frontend/pubspec.yaml" >&2
  FAIL=1
fi

if [[ $FAIL -ne 0 ]]; then
  exit 1
fi

if [[ -z "$TAG_VER" ]]; then
  echo "  ⚠️  当前仓库无 git tag，已跳过 tag 比对（CI 发布时会校验）"
fi
echo "  ✅ 版本号一致：$CHANGELOG_VER"
