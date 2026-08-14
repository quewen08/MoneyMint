#!/usr/bin/env bash
# P1-B 冒烟验证：迁移 002 + 注册语义 + 成员管理 + 删除语义 + 自然键合并 + 认证增强
# 用法：B=http://127.0.0.1:8123 bash export_demo/smoke_p1b.sh
set -u
B=${B:-http://127.0.0.1:8123}
PASS=0; FAIL=0
ck() { # ck <期望> <实际> <描述>
  if [ "$1" = "$2" ]; then PASS=$((PASS+1)); echo "  ✅ $3";
  else FAIL=$((FAIL+1)); echo "  ❌ $3 (期望 $1, 实际 $2)"; fi
}

echo "== 1. 注册语义 =="
R1=$(curl -s -X POST $B/api/auth/register -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"secret123","display_name":"Alice"}')
TOKEN1=$(echo "$R1" | sed 's/.*"token":"\([^"]*\)".*/\1/')
ck "owner" "$(echo "$R1" | sed 's/.*"role":"\([^"]*\)".*/\1/')" "首个注册用户成为 owner"
A1="Authorization: Bearer $TOKEN1"

curl -s -X POST $B/api/auth/register -H 'Content-Type: application/json' \
  -d '{"username":"bob","password":"secret123","display_name":"Bob"}' >/dev/null
TOKEN2=$(curl -s -X POST $B/api/auth/login -H 'Content-Type: application/json' \
  -d '{"username":"bob","password":"secret123"}' | sed 's/.*"token":"\([^"]*\)".*/\1/')
A2="Authorization: Bearer $TOKEN2"
ck "401" "$(curl -s -o /dev/null -w '%{http_code}' $B/api/auth/me)" "auth/me 未带 token → 401"
ck "200" "$(curl -s -o /dev/null -w '%{http_code}' $B/api/auth/me -H "$A2")" "bob auth/me（未加入账本也可用自服务端点）→ 200"
ck "403" "$(curl -s -o /dev/null -w '%{http_code}' $B/api/accounts -H "$A2")" "bob 未受邀访问业务端点 → 403"

echo "== 2. 成员管理（仅 owner） =="
ck "403" "$(curl -s -o /dev/null -w '%{http_code}' $B/api/ledger/members -H "$A2")" "bob 访问成员列表 → 403"
ck "200" "$(curl -s -o /dev/null -w '%{http_code}' $B/api/ledger/members -H "$A1")" "owner 访问成员列表 → 200"
curl -s -X POST $B/api/ledger/members -H "$A1" -H 'Content-Type: application/json' \
  -d '{"username":"bob","role":"editor"}' >/dev/null
ck "200" "$(curl -s -o /dev/null -w '%{http_code}' $B/api/accounts -H "$A2")" "bob 被加入后访问账户 → 200"
curl -s -X POST $B/api/ledger/members/2 -H "$A1" -H 'Content-Type: application/json' \
  -d '{"role":"viewer"}' >/dev/null
ck "viewer" "$(curl -s $B/api/auth/me -H "$A2" | sed 's/.*"role":"\([^"]*\)".*/\1/')" "owner 把 bob 改为 viewer"

echo "== 3. 删除语义（软删 + sync_log delete） =="
# 建账户 + 记账
ACC=$(curl -s -X POST $B/api/accounts -H "$A1" -H 'Content-Type: application/json' \
  -d '{"name":"Assets:Cash:CNY","type":"Assets","open_date":"2026-08-01","commodity":"CNY"}')
ACC_ID=$(echo "$ACC" | sed 's/.*"id":\([0-9]*\).*/\1/')
EXP=$(curl -s -X POST $B/api/accounts -H "$A1" -H 'Content-Type: application/json' \
  -d '{"name":"Expenses:Food:CNY","type":"Expenses","open_date":"2026-08-01","commodity":"CNY"}')
EXP_ID=$(echo "$EXP" | sed 's/.*"id":\([0-9]*\).*/\1/')
TXN=$(curl -s -X POST $B/api/transactions -H "$A1" -H 'Content-Type: application/json' \
  -d "{\"date\":\"2026-08-02\",\"description\":\"早餐\",\"flag\":\"*\",\"postings\":[{\"account_id\":$ACC_ID,\"commodity\":\"CNY\",\"amount\":\"-10.00\"},{\"account_id\":$EXP_ID,\"commodity\":\"CNY\",\"amount\":\"10.00\"}]}")
TXN_UUID=$(echo "$TXN" | sed 's/.*"uuid":"\([^"]*\)".*/\1/')
ck "200" "$(curl -s -o /dev/null -w '%{http_code}' -X DELETE $B/api/transactions/$TXN_UUID -H "$A1")" "DELETE /api/transactions/{uuid} → 200"
ck "404" "$(curl -s -o /dev/null -w '%{http_code}' -X DELETE $B/api/transactions/$TXN_UUID -H "$A1")" "重复删除（幂等）→ 404"

echo "== 4. sync/push：逐条 results + 自然键合并 =="
R=$(curl -s -X POST $B/api/sync/push -H "$A1" -H 'Content-Type: application/json' -d '{
  "client_id":"devA","since":0,
  "changes":[
    {"entity_type":"account","entity":{"uuid":"loc-x","name":"Assets:Bank:CNY","type":"Assets","open_date":"2026-08-01","commodity_restriction":"CNY"}},
    {"entity_type":"account","entity":{"uuid":"loc-x-dup","name":"Assets:Bank:CNY","type":"Assets","open_date":"2026-08-01","commodity_restriction":"CNY"}},
    {"entity_type":"transaction","entity":{"uuid":"loc-txn-bad","date":"2026-08-03","flag":"*","description":"引用缺失","postings":[{"account_uuid":"nope","commodity":"CNY","amount":"-5.00"},{"account_uuid":"loc-x","commodity":"CNY","amount":"5.00"}]}}
  ]
}')
echo "$R"
# 逐条校验（用 python 解析 JSON，避免 sed 误配）
# accepted=1：仅 loc-x 真正插入；loc-x-dup 同名合并（未插入）；条目2 引用缺失失败
ck "1" "$(echo "$R" | python -c "import sys,json; d=json.load(sys.stdin); print(d['accepted'])")" "push accepted=1（仅 loc-x 真正插入，合并/失败不计入）"
ck "ok" "$(echo "$R" | python -c "import sys,json; d=json.load(sys.stdin); print([r['status'] for r in d['results']][0])")" "条目0 账户创建 ok"
ck "error" "$(echo "$R" | python -c "import sys,json; d=json.load(sys.stdin); print([r['status'] for r in d['results']][2])")" "条目2 引用缺失 → error（单条失败不阻塞其余）"
SU=$(echo "$R" | python -c "import sys,json; d=json.load(sys.stdin); print([r.get('server_uuid') for r in d['results']][1])")
ck "loc-x" "$SU" "条目1 同名账户自然键合并 → server_uuid=先到者 loc-x"

echo "== 5. 认证增强：change-password / refresh / owner 重置成员密码 =="
curl -s -X POST $B/api/auth/change-password -H "$A1" -H 'Content-Type: application/json' \
  -d '{"old_password":"secret123","new_password":"newpass123"}' >/dev/null
ck "401" "$(curl -s -o /dev/null -w '%{http_code}' $B/api/auth/me -H "$A1")" "改密后旧 token 失效 → 401"
TOKEN1N=$(curl -s -X POST $B/api/auth/login -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"newpass123"}' | sed 's/.*"token":"\([^"]*\)".*/\1/')
A1N="Authorization: Bearer $TOKEN1N"
ck "200" "$(curl -s -o /dev/null -w '%{http_code}' $B/api/auth/me -H "$A1N")" "新密码登录 → 200"
# refresh 会使旧 token 立即失效，必须取新 token 继续用
TOKEN1R=$(curl -s -X POST $B/api/auth/refresh -H "$A1N" | sed 's/.*"token":"\([^"]*\)".*/\1/')
A1R="Authorization: Bearer $TOKEN1R"
ck "200" "$(curl -s -o /dev/null -w '%{http_code}' $B/api/auth/me -H "$A1R")" "refresh 后新 token 可用 → 200"
ck "401" "$(curl -s -o /dev/null -w '%{http_code}' $B/api/auth/me -H "$A1N")" "refresh 后旧 token 失效 → 401"
ck "200" "$(curl -s -o /dev/null -w '%{http_code}' -X POST $B/api/ledger/members/2/reset-password -H "$A1R" -H 'Content-Type: application/json' -d '{"new_password":"bob123456"}')" "owner 重置 bob 密码 → 200"

echo "== 6. 导出 bean-check 仍通过 =="
curl -s $B/api/export -H "$A1R" -o data/p1b_verify.beancount
ck "0" "$(python -c "print(1 if open('data/p1b_verify.beancount').read(1)=='{' else 0)")" "导出非 JSON 错误（首字符不是 {）"
cd "C:/Users/FLASHBASE/WorkBuddy/2026-08-10-16-07-53" && .venv/Scripts/bean-check.exe data/p1b_verify.beancount >/dev/null 2>&1
ck "0" "$?" "bean-check 退出码"

echo
echo "PASS=$PASS FAIL=$FAIL"
[ "$FAIL" = "0" ] && exit 0 || exit 1
