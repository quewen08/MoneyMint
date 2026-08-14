#!/usr/bin/env bash
# P1-B 后端冒烟测试（全新库 data/p1b_smoke.db，端口 8100）
# 覆盖：B3 注册语义/成员管理/改密/刷新、B1 delete 语义、B2 push 逐条、自然键合并、bean-check。
set -u
cd "$(dirname "$0")/.."
export PATH="$PATH:/d/Environment/go/bin"

PORT=8100
BASE="http://127.0.0.1:$PORT"
DB=data/p1b_smoke.db
rm -f "$DB" "$DB-shm" "$DB-wal"

PASS=0
FAIL=0
check() { # $1=描述 $2=期望包含 $3=实际
  if echo "$3" | grep -q "$2"; then
    echo "  ✅ $1"
    PASS=$((PASS+1))
  else
    echo "  ❌ $1  期望含 [$2] 实际 [$3]"
    FAIL=$((FAIL+1))
  fi
}
code() { # $1=URL $2=方法 $3=token(可空) $4=body(可空) → 输出 HTTP 状态码
  local tok=""; [ -n "${3:-}" ] && tok="-H \"Authorization: Bearer $3\""
  local body=""; [ -n "${4:-}" ] && body="-d '$4'"
  eval "curl -s -o /dev/null -w '%{http_code}' -X $2 \"$1\" -H 'Content-Type: application/json' $tok $body"
}

echo "== 启动后端 :$PORT =="
LEDGER_DB="$DB" LISTEN=":$PORT" go run ./cmd/server >/tmp/p1b_server.log 2>&1 &
SRV=$!
for i in $(seq 1 40); do
  curl -s "$BASE/api/health" >/dev/null 2>&1 && break
  sleep 0.3
done

echo "== 1. 注册语义 =="
R1=$(curl -s -X POST "$BASE/api/auth/register" -H 'Content-Type: application/json' -d '{"username":"alice","password":"secret123","display_name":"Alice"}')
TA=$(echo "$R1" | sed 's/.*"token":"\([^"]*\)".*/\1/')
check "首个注册 alice 为 owner" '"role":"owner"' "$R1"

R2=$(curl -s -X POST "$BASE/api/auth/register" -H 'Content-Type: application/json' -d '{"username":"bob","password":"secret456","display_name":"Bob"}')
check "次个注册 bob 仅建账号(role 空)" '"role":""' "$R2"
TB=$(echo "$R2" | sed 's/.*"token":"\([^"]*\)".*/\1/')

echo "== 2. 非成员 403 / owner 建账记账 =="
B1=$(code "$BASE/api/accounts" GET "$TB")
check "bob 未入账本访问账户 403" '403' "$B1"

A1=$(curl -s -X POST "$BASE/api/accounts" -H "Authorization: Bearer $TA" -H 'Content-Type: application/json' \
  -d '{"name":"Assets:Cash:CNY","type":"Assets","open_date":"2026-08-01","commodity":"CNY"}')
check "alice 建账户" '"uuid":"' "$A1"
A1UUID=$(echo "$A1" | sed 's/.*"uuid":"\([^"]*\)".*/\1/')
A1ID=$(echo "$A1" | sed 's/.*"id":\([0-9]*\).*/\1/')

A2=$(curl -s -X POST "$BASE/api/accounts" -H "Authorization: Bearer $TA" -H 'Content-Type: application/json' \
  -d '{"name":"Expenses:Food","type":"Expenses","open_date":"2026-08-01","commodity":"CNY"}')
A2UUID=$(echo "$A2" | sed 's/.*"uuid":"\([^"]*\)".*/\1/')
A2ID=$(echo "$A2" | sed 's/.*"id":\([0-9]*\).*/\1/')

TX=$(curl -s -X POST "$BASE/api/transactions" -H "Authorization: Bearer $TA" -H 'Content-Type: application/json' \
  -d "{\"date\":\"2026-08-02\",\"description\":\"早餐\",\"flag\":\"*\",\"postings\":[{\"account_id\":$A1ID,\"commodity\":\"CNY\",\"amount\":\"-10.00\"},{\"account_id\":$A2ID,\"commodity\":\"CNY\",\"amount\":\"10.00\"}]}")
check "在线记账" '"uuid":"' "$TX"
TXUUID=$(echo "$TX" | sed 's/.*"uuid":"\([^"]*\)".*/\1/')

echo "== 3. 成员管理（B3） =="
M1=$(curl -s -X POST "$BASE/api/ledger/members" -H "Authorization: Bearer $TA" -H 'Content-Type: application/json' \
  -d '{"username":"bob","role":"editor"}')
check "owner 添加 bob 为 editor" '"role":"editor"' "$M1"
ML=$(curl -s "$BASE/api/ledger/members" -H "Authorization: Bearer $TA")
check "成员列表含 bob" '"username":"bob"' "$ML"
MLB=$(code "$BASE/api/ledger/members" GET "$TB")
check "非 owner 看成员列表 403" '403' "$MLB"
B2=$(curl -s "$BASE/api/accounts" -H "Authorization: Bearer $TB")
check "bob 入账本后可见账户" '"name":"Assets:Cash:CNY"' "$B2"

echo "== 4. 删除语义（B1） =="
D1=$(curl -s -X DELETE "$BASE/api/transactions/$TXUUID" -H "Authorization: Bearer $TA")
check "删除交易" '"ok":true' "$D1"
BAL=$(curl -s "$BASE/api/accounts" -H "Authorization: Bearer $TA")
if echo "$BAL" | grep -q '"balances"'; then
  echo "  ❌ 删除交易后余额字段仍存在: $BAL"; FAIL=$((FAIL+1))
else
  echo "  ✅ 删除交易后余额归零(balances 字段省略)"; PASS=$((PASS+1))
fi

# 再记一笔后删除账户（连带删交易）
TX2=$(curl -s -X POST "$BASE/api/transactions" -H "Authorization: Bearer $TA" -H 'Content-Type: application/json' \
  -d "{\"date\":\"2026-08-03\",\"description\":\"午餐\",\"flag\":\"*\",\"postings\":[{\"account_id\":$A1ID,\"commodity\":\"CNY\",\"amount\":\"-20.00\"},{\"account_id\":$A2ID,\"commodity\":\"CNY\",\"amount\":\"20.00\"}]}")
DA=$(curl -s -X DELETE "$BASE/api/accounts/$A2UUID" -H "Authorization: Bearer $TA")
check "删除账户(连带交易)" '"ok":true' "$DA"
DA2=$(curl -s -X DELETE "$BASE/api/accounts/$A2UUID" -H "Authorization: Bearer $TA")
check "重复删除幂等 404" '404' "$(code "$BASE/api/accounts/$A2UUID" DELETE "$TA")"

echo "== 5. pull 含 delete 事件 =="
PULL=$(curl -s "$BASE/api/sync/pull?since=0&client_id=smokeA" -H "Authorization: Bearer $TA")
check "pull 含 delete 事件" '"op":"delete"' "$PULL"

echo "== 6. push 逐条提交（B2）+ 自然键合并 =="
# 两条交易：第一条引用缺失账户(loc-missing)，第二条正常引用
BODY='{"client_id":"smokeB","since":0,"changes":[
  {"entity_type":"transaction","entity":{"uuid":"loc-txn-bad","date":"2026-08-04","flag":"*","description":"坏交易",
    "postings":[{"account_uuid":"loc-missing","commodity":"CNY","amount":"-1.00"},{"account_uuid":"loc-ok","commodity":"CNY","amount":"1.00"}]}},
  {"entity_type":"account","entity":{"uuid":"loc-ok","name":"Assets:Bank","type":"Assets","open_date":"2026-08-01","commodity_restriction":"CNY"}},
  {"entity_type":"account","entity":{"uuid":"loc-ok2","name":"Expenses:Bank","type":"Expenses","open_date":"2026-08-01","commodity_restriction":"CNY"}}
]}'
PUSH=$(curl -s -X POST "$BASE/api/sync/push" -H "Authorization: Bearer $TA" -H 'Content-Type: application/json' -d "$BODY")
check "坏交易返回 error(引用缺失)" '"status":"error"' "$PUSH"
check "账户创建成功" '"status":"ok"' "$PUSH"

# 自然键合并：推送与已存在同名的账户（不同 loc-uuid）
BODY2='{"client_id":"smokeB","since":0,"changes":[
  {"entity_type":"account","entity":{"uuid":"loc-cash-copy","name":"Assets:Cash:CNY","type":"Assets","open_date":"2026-08-01"}}
]}'
PUSH2=$(curl -s -X POST "$BASE/api/sync/push" -H "Authorization: Bearer $TA" -H 'Content-Type: application/json' -d "$BODY2")
check "同名账户自然键合并返回先到者 uuid" "$A1UUID" "$PUSH2"
check "合并条目 accepted=false" '"accepted":false' "$PUSH2"

echo "== 7. 导出 + bean-check =="
curl -s "$BASE/api/export" -H "Authorization: Bearer $TA" -o data/p1b_family.beancount
.venv/Scripts/bean-check.exe data/p1b_family.beancount >/dev/null 2>&1
check "导出 bean-check 通过" "0" "$?"
if grep -q "Expenses:Food" data/p1b_family.beancount; then
  echo "  ❌ 导出仍含已删账户 Expenses:Food"; FAIL=$((FAIL+1))
else
  echo "  ✅ 导出不含已删账户"; PASS=$((PASS+1))
fi

echo "== 8. 改密 / 刷新 / owner 重置 =="
CP=$(curl -s -X POST "$BASE/api/auth/change-password" -H "Authorization: Bearer $TA" -H 'Content-Type: application/json' \
  -d '{"old_password":"secret123","new_password":"newpass789"}')
check "改密成功返回新 token" '"ok":true' "$CP"
TNEW=$(echo "$CP" | sed 's/.*"token":"\([^"]*\)".*/\1/')
check "旧 token 已失效 401" '401' "$(code "$BASE/api/auth/me" GET "$TA")"
check "新 token 可用" '"username":"alice"' "$(curl -s "$BASE/api/auth/me" -H "Authorization: Bearer $TNEW")"

RF=$(curl -s -X POST "$BASE/api/auth/refresh" -H "Authorization: Bearer $TNEW")
check "刷新 token 成功" '"token":"' "$RF"
# refresh 前的 token（即改密后拿到的 $TNEW）应已被服务端删除
check "刷新后旧 token 失效 401" '401' "$(code "$BASE/api/auth/me" GET "$TNEW")"
TNEW=$(echo "$RF" | sed 's/.*"token":"\([^"]*\)".*/\1/')
check "刷新后新 token 可用" '"username":"alice"' "$(curl -s "$BASE/api/auth/me" -H "Authorization: Bearer $TNEW")"

RP=$(curl -s -X POST "$BASE/api/ledger/members/2/reset-password" -H "Authorization: Bearer $TNEW" -H 'Content-Type: application/json' \
  -d '{"new_password":"bobnew789"}')
check "owner 重置 bob 密码" '"ok":true' "$RP"
check "bob 旧 token 失效 401" '401' "$(code "$BASE/api/auth/me" GET "$TB")"
BLOG=$(curl -s -X POST "$BASE/api/auth/login" -H 'Content-Type: application/json' -d '{"username":"bob","password":"bobnew789"}')
check "bob 新密码可登录" '"token":"' "$BLOG"

kill $SRV 2>/dev/null
echo ""
echo "结果：PASS=$PASS FAIL=$FAIL"
[ $FAIL -eq 0 ]
