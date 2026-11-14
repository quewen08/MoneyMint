#!/usr/bin/env bash
# 导出兼容性校验入口：确认 bean-check 可用，并用最小账本自检。
#
# 用途：
#   - CI 中作为独立一步，防止 `pip install beancount` 悄悄失败导致后续校验被跳过
#   - 本地 `make check` 调用
#
# 退出码：0 = 校验器可用且自检通过；1 = 不可用或自检失败
#
# 注意：本脚本只验证「校验器本身可用」，不验证业务账本。
# 业务账本的校验在 go test -run TestExport 中进行（需 BEAN_CHECK_STRICT=1）。

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

# 候选校验器：项目内 venv（两种平台布局）→ 系统 python → PATH 上的 bean-check
find_checker() {
  local cands=(
    "$ROOT_DIR/.venv/Scripts/python.exe|-m|beancount.scripts.check"
    "$ROOT_DIR/.venv/bin/python|-m|beancount.scripts.check"
    "python3|-m|beancount.scripts.check"
    "python|-m|beancount.scripts.check"
    "bean-check||"
  )
  for entry in "${cands[@]}"; do
    IFS='|' read -r exe a1 a2 <<< "$entry"
    # 带路径的候选先确认文件存在
    if [[ "$exe" == */* || "$exe" == *\\* ]]; then
      [[ -x "$exe" ]] || continue
    else
      command -v "$exe" >/dev/null 2>&1 || continue
    fi
    # 用最小账本试跑，能跑通才认为可用（排除「python 在但没装 beancount」）
    local probe
    probe="$(mktemp -d)"
    cat > "$probe/probe.bean" <<'BEAN'
2026-01-01 open Assets:Probe CNY
2026-01-01 open Expenses:Probe CNY
2026-01-02 * "probe"
  Assets:Probe  -1.00 CNY
  Expenses:Probe  1.00 CNY
BEAN
    if "$exe" ${a1:+"$a1"} ${a2:+"$a2"} "$probe/probe.bean" >/dev/null 2>&1; then
      rm -rf "$probe"
      echo "$exe|${a1}|${a2}"
      return 0
    fi
    rm -rf "$probe"
  done
  return 1
}

if ! checker="$(find_checker)"; then
  echo "❌ 未找到可用的 bean-check（beancount 未安装）" >&2
  echo "   安装方式：" >&2
  echo "     python -m venv .venv" >&2
  echo "     激活 .venv 后：python -m pip install beancount" >&2
  exit 1
fi

IFS='|' read -r EXE A1 A2 <<< "$checker"
echo "✅ 校验器可用：$EXE ${A1:-} ${A2:-}"

# 用一份覆盖主要指令的最小账本做端到端自检
WORK="$(mktemp -d)"
cleanup() { rm -rf "$WORK"; }
trap cleanup EXIT

cat > "$WORK/selfcheck.bean" <<'BEAN'
option "title" "self-check"
option "operating_currency" "CNY"

2026-01-01 commodity CNY
  name: "人民币"

2026-01-01 open Assets:Cash:CNY CNY
2026-01-01 open Expenses:Food CNY
2026-01-01 open Equity:Opening-Balances CNY

2026-01-01 * "期初余额"
  Assets:Cash:CNY            1000.00 CNY
  Equity:Opening-Balances   -1000.00 CNY

2026-01-02 * "带标签的午餐"
  tags: "餐饮 出差"
  Assets:Cash:CNY             -38.50 CNY
  Expenses:Food                38.50 CNY
BEAN

if ! "$EXE" ${A1:+"$A1"} ${A2:+"$A2"} "$WORK/selfcheck.bean"; then
  echo "❌ bean-check 自检失败：最小账本未通过校验，说明校验器或导出语法存在根本问题" >&2
  exit 1
fi

echo "✅ bean-check 自检通过（含 commodity / open / 期初余额 / 标签元数据）"
