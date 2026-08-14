#!/usr/bin/env bash
# 模拟前端 SyncService 两轮 push 序列（自然键合并 → 本地 remap → 重推交易）。
# 验证：账户同名合并返回 server_uuid；引用 loc-uuid 的交易第一轮 error；
# 本地 remap 为 server_uuid 后第二轮 push 成功。
set -u
cd "$(dirname "$0")/.."
export PATH="$PATH:/d/Environment/go/bin"

PORT=8101
BASE="http://127.0.0.1:$PORT"
DB=data/p1b_2round.db
rm -f "$DB" "$DB-shm" "$DB-wal"

PASS=0
FAIL=0
check() { # $1=描述 $2=期望包含 $3=实际
  if echo "$3" | grep -q "$2"; then
    echo "  ✅ $1"; PASS=$((PASS+1))
  else
    echo "  ❌ $1  期望含 [$2] 实际 [$3]"; FAIL=$((FAIL+1))
  fi
}

echo "== 启动后端 :$PORT =="
LEDGER_DB="$DB" LISTEN=":$PORT" go run ./cmd/server >/tmp/p1b_2round.log 2>&1 &
SRV=$!
for i in $(seq 1 40); do
  curl -s "$BASE/api/health" >/dev/null 2>&1 && break
  sleep 0.3
done

R=$(curl -s -X POST "$BASE/api/auth/register" -H 'Content-Type: application/json' -d '{"username":"owner","password":"secret123"}')
T=$(echo "$R" | sed 's/.*"token":"\([^"]*\)".*/\1/')

# 在线建一个账户（即服务器上「先到者」），拿到真实 uuid。
A=$(curl -s -X POST "$BASE/api/accounts" -H "Authorization: Bearer $T" -H 'Content-Type: application/json' \
  -d '{"name":"Assets:Cash:CNY","type":"Assets","open_date":"2026-08-01","commodity":"CNY"}')
SERVER_ACC=$(echo "$A" | sed 's/.*"uuid":"\([^"]*\)".*/\1/')
echo "服务端先到者账户 uuid: $SERVER_ACC"

echo "== 第 1 轮 push：账户 loc-cash 与 acc-5 同名 + 交易引用 loc-cash =="
BODY1='{"client_id":"webA","since":0,"changes":[
  {"entity_type":"account","entity":{"uuid":"loc-cash","name":"Assets:Cash:CNY","type":"Assets","open_date":"2026-08-01"}},
  {"entity_type":"transaction","entity":{"uuid":"loc-txn1","date":"2026-08-02","flag":"*","description":"早餐",
    "postings":[{"account_uuid":"loc-cash","commodity":"CNY","amount":"-10.00"},{"account_uuid":"loc-food","commodity":"CNY","amount":"10.00"}]}},
  {"entity_type":"account","entity":{"uuid":"loc-food","name":"Expenses:Food","type":"Expenses","open_date":"2026-08-01"}}
]}'
P1=$(curl -s -X POST "$BASE/api/sync/push" -H "Authorization: Bearer $T" -H 'Content-Type: application/json' -d "$BODY1")
echo "$P1" | python -m json.tool 2>/dev/null | head -60
check "账户 loc-cash 被合并返回 server_uuid" "$SERVER_ACC" "$P1"
check "交易 loc-txn1 第一轮 error（引用 loc-cash 不存在）" '"status":"error"' "$P1"
check "账户 loc-food 创建成功" '"accepted":true' "$P1"

echo "== 第 2 轮 push：本地 remap 后重推交易（引用已改 server_uuid） =="
BODY2='{"client_id":"webA","since":0,"changes":[
  {"entity_type":"transaction","entity":{"uuid":"loc-txn1","date":"2026-08-02","flag":"*","description":"早餐",
    "postings":[{"account_uuid":"'$SERVER_ACC'","commodity":"CNY","amount":"-10.00"},{"account_uuid":"loc-food","commodity":"CNY","amount":"10.00"}]}}
]}'
P2=$(curl -s -X POST "$BASE/api/sync/push" -H "Authorization: Bearer $T" -H 'Content-Type: application/json' -d "$BODY2")
check "交易 loc-txn1 第二轮成功" '"status":"ok"' "$P2"

echo "== pull 验证：loc-cash 无 create 事件（合并未落库），loc-food/loc-txn1 有 create =="
PULL=$(curl -s "$BASE/api/sync/pull?since=0&client_id=webA" -H "Authorization: Bearer $T")
check "pull 含 loc-txn1 create" '"uuid":"loc-txn1"' "$PULL"
if echo "$PULL" | grep -q 'loc-cash'; then
  echo "  ❌ pull 不应出现 loc-cash（合并未落库）"; FAIL=$((FAIL+1))
else
  echo "  ✅ pull 不含 loc-cash（合并未落库）"; PASS=$((PASS+1))
fi

echo "== 在线列表：loc-txn1 的 postings 引用先到者账户（列表接口内联账户名） =="
TXNS=$(curl -s "$BASE/api/transactions" -H "Authorization: Bearer $T")
check "在线交易存在（早餐）" '早餐' "$TXNS"
check "postings 引用先到者账户 Assets:Cash:CNY" 'Assets:Cash:CNY' "$TXNS"
if echo "$TXNS" | grep -q 'Expenses:Food'; then
  echo "  ✅ 同时引用新建账户 Expenses:Food"; PASS=$((PASS+1))
else
  echo "  ❌ 未引用新建账户 Expenses:Food"; FAIL=$((FAIL+1))
fi

kill $SRV 2>/dev/null
echo ""
echo "结果：PASS=$PASS FAIL=$FAIL"
[ $FAIL -eq 0 ]
