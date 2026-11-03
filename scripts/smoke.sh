#!/usr/bin/env bash
# 端到端冒烟：起服务 → 健康检查 → 注册 → 建账户 → 记账 → 导出 → bean-check → 备份。
#
# 用法：bash scripts/smoke.sh [端口]
#
# 设计说明：
#   - 用临时数据库与临时端口，不污染 data/ledger.db
#   - 备份相关步骤会先探测端点是否存在：M1 未落地时自动跳过而非报错，
#     这样脚本在任意阶段都能跑，落地后自动覆盖更多链路
#   - 失败时保留临时目录并打印位置，便于排查

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if ! command -v go >/dev/null 2>&1 && [[ -x /d/Environment/go/bin/go ]]; then
  export PATH="$PATH:/d/Environment/go/bin"
fi

PORT="${1:-${SMOKE_PORT:-18099}}"
BASE="http://127.0.0.1:${PORT}"
WORK="$(mktemp -d)"
SERVER_PID=""
KEEP_WORK=0

cleanup() {
  if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" 2>/dev/null; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  if [[ $KEEP_WORK -eq 0 ]]; then
    # Windows 下进程刚退出时二进制文件的句柄可能尚未释放，重试后再放弃
    rm -rf "$WORK" 2>/dev/null \
      || { sleep 1; rm -rf "$WORK" 2>/dev/null; } \
      || true
  fi
}
trap cleanup EXIT

pass() { echo "  ✅ $1"; }
fail() {
  echo "  ❌ $1" >&2
  KEEP_WORK=1
  echo "     现场保留在：$WORK" >&2
  [[ -f "$WORK/server.log" ]] && { echo "     ---- server.log ----" >&2; tail -30 "$WORK/server.log" >&2; }
  exit 1
}

# 从 JSON 里取字符串字段（不依赖 jq）
json_str() {
  sed -n "s/.*\"$1\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" <<< "$2" | head -1
}
json_num() {
  sed -n "s/.*\"$1\"[[:space:]]*:[[:space:]]*\([0-9][0-9]*\).*/\1/p" <<< "$2" | head -1
}

echo "== 1. 编译并启动服务 =="
mkdir -p "$WORK"
CGO_ENABLED=0 go build -o "$WORK/server" ./cmd/server || fail "编译失败"
LEDGER_DB="$WORK/smoke.db" \
LEDGER_WEB_ROOT="$WORK/web" \
LEDGER_BACKUP_DIR="$WORK/backups" \
LISTEN=":$PORT" \
"$WORK/server" > "$WORK/server.log" 2>&1 &
SERVER_PID=$!
pass "编译完成，PID=$SERVER_PID"

ready=0
for _ in $(seq 1 40); do
  if curl -fsS -m 2 "$BASE/api/health" >/dev/null 2>&1; then ready=1; break; fi
  sleep 0.5
done
[[ $ready -eq 1 ]] || fail "服务未在 20 秒内就绪"

echo "== 2. 健康检查 =="
HEALTH="$(curl -fsS -m 5 "$BASE/api/health")"
[[ "$HEALTH" == *'"status":"ok"'* ]] || fail "health 响应异常：$HEALTH"
pass "GET /api/health → $HEALTH"

echo "== 3. 注册与身份 =="
USER="smoke_$$"
REG="$(curl -fsS -m 10 -X POST "$BASE/api/auth/register" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"$USER\",\"password\":\"${SMOKE_PASSWORD:-smoke-pass-123456}\",\"display_name\":\"冒烟用户\"}")"
TOKEN="$(json_str token "$REG")"
[[ -n "$TOKEN" ]] || fail "注册未返回 token：$REG"
AUTH="Authorization: Bearer $TOKEN"
pass "注册成功（username=$USER）"

ME="$(curl -fsS -m 5 "$BASE/api/auth/me" -H "$AUTH")"
pass "GET /api/auth/me → $ME"
if [[ "$ME" == *'"is_admin":true'* ]]; then
  pass "首个用户自动成为管理员"
else
  echo "  ⚠️  me 未返回 is_admin=true（M1-a 未落地时属正常）"
fi

echo "== 4. 建账户 =="
# 注意：首个账本会自动 seed 一批默认账户（含 Expenses:Food 等），
# 这里用带 Smoke 前缀的独立账户，避免与种子数据撞名导致 409。
mkacct() {
  curl -fsS -m 10 -X POST "$BASE/api/accounts" -H "$AUTH" -H 'Content-Type: application/json' \
    -d "$1"
}
A1="$(mkacct '{"name":"Assets:Smoke:Cash","type":"Assets","open_date":"2026-11-01","commodity":"CNY"}')"
A2="$(mkacct '{"name":"Expenses:Smoke:Food","type":"Expenses","open_date":"2026-11-01","commodity":"CNY"}')"
ID1="$(json_num id "$A1")"
ID2="$(json_num id "$A2")"
[[ -n "$ID1" && -n "$ID2" ]] || fail "建账户失败：$A1 / $A2"
pass "资产账户 id=$ID1，支出账户 id=$ID2"

echo "== 5. 记一笔（借贷平衡） =="
# 注意：REST 端点 POST /api/transactions 的请求结构不支持 tags（仅 sync/push 路径支持），
# 且 handler 用了 DisallowUnknownFields，多传未知字段会直接 400。此处不传 tags。
TXN="$(curl -fsS -m 10 -X POST "$BASE/api/transactions" -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"date\":\"2026-11-02\",\"description\":\"冒烟早餐\",\"postings\":[
        {\"account_id\":$ID1,\"commodity\":\"CNY\",\"amount\":\"-12.50\"},
        {\"account_id\":$ID2,\"commodity\":\"CNY\",\"amount\":\"12.50\"}]}")"
pass "记账成功"

echo "== 6. 不平衡交易应被拒绝 =="
CODE="$(curl -s -o /dev/null -w '%{http_code}' -m 10 -X POST "$BASE/api/transactions" \
  -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"date\":\"2026-11-02\",\"description\":\"不平衡\",\"postings\":[
        {\"account_id\":$ID1,\"commodity\":\"CNY\",\"amount\":\"-12.50\"},
        {\"account_id\":$ID2,\"commodity\":\"CNY\",\"amount\":\"10.00\"}]}")"
[[ "$CODE" =~ ^4 ]] || fail "不平衡交易未被拒绝（HTTP $CODE）"
pass "不平衡交易被拒绝（HTTP $CODE）"

echo "== 7. 未登录访问必须 401 =="
CODE="$(curl -s -o /dev/null -w '%{http_code}' -m 5 "$BASE/api/accounts")"
[[ "$CODE" == "401" ]] || fail "未登录访问 /api/accounts 期望 401，实际 $CODE"
pass "未登录 → 401"

echo "== 8. 单文件导出 + bean-check =="
curl -fsS -m 10 "$BASE/api/export" -H "$AUTH" -o "$WORK/ledger.beancount" || fail "导出失败"
[[ -s "$WORK/ledger.beancount" ]] || fail "导出文件为空"
pass "导出 $(wc -c < "$WORK/ledger.beancount" | tr -d ' ') 字节"

echo "== 9. 多文件目录导出 + bean-check =="
curl -fsS -m 15 "$BASE/api/export/archive" -H "$AUTH" -o "$WORK/archive.zip" || fail "目录导出失败"
mkdir -p "$WORK/archive"
if command -v unzip >/dev/null 2>&1; then
  unzip -q "$WORK/archive.zip" -d "$WORK/archive"
  [[ -f "$WORK/archive/main.bean" ]] || fail "压缩包内缺少 main.bean"
  pass "解压后 main.bean 存在（$(find "$WORK/archive" -name '*.bean' | wc -l | tr -d ' ') 个 .bean 文件）"
else
  echo "  ⚠️  未找到 unzip，跳过目录内容校验"
fi

# bean-check：优先项目 venv
BEAN=""
for cand in "$ROOT_DIR/.venv/Scripts/python.exe" "$ROOT_DIR/.venv/bin/python"; do
  if [[ -x "$cand" ]] && "$cand" -c "import beancount" >/dev/null 2>&1; then BEAN="$cand -m beancount.scripts.check"; break; fi
done
if [[ -z "$BEAN" ]] && command -v python3 >/dev/null 2>&1 && python3 -c "import beancount" >/dev/null 2>&1; then
  BEAN="python3 -m beancount.scripts.check"
fi

if [[ -n "$BEAN" ]]; then
  $BEAN "$WORK/ledger.beancount" || fail "单文件导出未通过 bean-check"
  pass "单文件导出通过 bean-check"
  if [[ -f "$WORK/archive/main.bean" ]]; then
    ( cd "$WORK/archive" && $BEAN main.bean ) || fail "目录导出未通过 bean-check"
    pass "目录导出通过 bean-check"
  fi
else
  echo "  ⚠️  未找到可用的 beancount，跳过 bean-check（CI 中为强制项）"
fi

echo "== 10. 备份端点（M1 未落地时跳过） =="
BCODE="$(curl -s -o /dev/null -w '%{http_code}' -m 10 "$BASE/api/backup/list" -H "$AUTH")"
if [[ "$BCODE" == "200" ]]; then
  RUN="$(curl -fsS -m 30 -X POST "$BASE/api/backup/run" -H "$AUTH")"
  BNAME="$(json_str name "$RUN")"
  [[ -n "$BNAME" ]] || fail "备份未返回文件名：$RUN"
  pass "手动备份成功：$BNAME"

  curl -fsS -m 30 "$BASE/api/backup/download?name=$BNAME" -H "$AUTH" -o "$WORK/dl.db" || fail "备份下载失败"
  [[ -s "$WORK/dl.db" ]] || fail "下载的备份为空"
  pass "备份下载 $(wc -c < "$WORK/dl.db" | tr -d ' ') 字节"

  # 路径穿越防护
  TC="$(curl -s -o /dev/null -w '%{http_code}' -m 10 \
        "$BASE/api/backup/download?name=../../etc/passwd" -H "$AUTH")"
  [[ "$TC" == "400" ]] || fail "路径穿越未被拒绝（HTTP $TC）"
  pass "路径穿越被拒绝（HTTP 400）"
else
  echo "  ⚠️  /api/backup/list 返回 $BCODE，M1 尚未落地，跳过"
fi

echo ""
echo "🎉 冒烟全部通过"
