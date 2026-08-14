#!/usr/bin/env bash
# 模拟「家庭多人 + 多端离线」同步：设备A在线记账、设备B拉取合并、
# 设备B离线记账后推送回服务端、设备A再次拉取合并、幂等性验证。
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="$PATH:/d/Environment/go/bin"

# 选一个空闲端口，避免与本机残留服务/孤儿进程撞车。
# 注意：Windows 上 Go 的 TCP 监听默认 SO_REUSEADDR，多个进程可能共用同一端口，
# 导致连接被分摊到不同（旧）实例而偶发 401，因此这里每次用随机空闲端口最稳妥。
PORT=$(python3 -c "import socket;s=socket.socket();s.bind(('127.0.0.1',0));print(s.getsockname()[1]);s.close()")
B="http://127.0.0.1:$PORT"
# 尽力清理占用该端口的残留进程（best-effort，处理常见的单孤儿情形）
LPID=$(netstat -ano 2>/dev/null | grep ":$PORT " | grep LISTENING | awk '{print $5}' | head -1 || true)
if [ -n "$LPID" ]; then taskkill.exe /PID "$LPID" /F >/dev/null 2>&1 || true; sleep 1; fi
DB="data/sync_test_$(date +%s).db"
rm -f "$DB" "$DB-wal" "$DB-shm" 2>/dev/null || true
export LEDGER_DB="$DB"
export LISTEN=":$PORT"

echo "=== 编译并启动服务端（后台）==="
go build -o ./.sync_test_bin ./cmd/server || { echo "编译失败"; exit 1; }
./.sync_test_bin > /tmp/sync_server.log 2>&1 &
SRV=$!
for i in $(seq 1 25); do curl -s -o /dev/null "$B/api/health" && break; sleep 1; done
curl -s "$B/api/health"; echo

# --- 认证：注册并登录拿到 token（所有业务请求需带 Authorization 头）---
echo; echo "=== 注册并登录同步测试用户 ==="
curl -s -o /dev/null -X POST "$B/api/auth/register" -H 'Content-Type: application/json' \
  -d '{"username":"synctester","password":"secret123","display_name":"SyncTester"}' || true
TOKEN=$(curl -s -X POST "$B/api/auth/login" -H 'Content-Type: application/json' \
  -d '{"username":"synctester","password":"secret123"}' | python3 -c "import sys,json;print(json.load(sys.stdin)['token'])")
# 用数组存 -H 与头值两个词，展开时保持原子，避免分词把 token 拆成多余参数导致 401。
AUTH_HDR=(-H "Authorization: Bearer $TOKEN")
echo "token=${TOKEN:0:16}..."

# --- 设备A：在线创建账户与交易 ---
echo; echo "=== 设备A：创建账户 Assets:Cash:CNY / Expenses:Food / Equity:Opening ==="
CA=$(curl -s -X POST "$B/api/accounts" -H 'Content-Type: application/json' "${AUTH_HDR[@]}" \
  -d '{"name":"Assets:Cash:CNY","type":"Assets","open_date":"2026-08-01","commodity":"CNY"}')
CF=$(curl -s -X POST "$B/api/accounts" -H 'Content-Type: application/json' "${AUTH_HDR[@]}" \
  -d '{"name":"Expenses:Food","type":"Expenses","open_date":"2026-08-01"}')
CE=$(curl -s -X POST "$B/api/accounts" -H 'Content-Type: application/json' "${AUTH_HDR[@]}" \
  -d '{"name":"Equity:Opening:Balances","type":"Equity","open_date":"2026-08-01"}')
UUID_CASH=$(echo "$CA" | python3 -c "import sys,json;print(json.load(sys.stdin)['uuid'])")
UUID_FOOD=$(echo "$CF" | python3 -c "import sys,json;print(json.load(sys.stdin)['uuid'])")
ID_EQUITY=$(echo "$CE" | python3 -c "import sys,json;print(json.load(sys.stdin)['id'])")
echo "cash uuid=$UUID_CASH  food uuid=$UUID_FOOD  equity id=$ID_EQUITY"

echo "=== 设备A：记 opening 交易 ==="
curl -s -X POST "$B/api/transactions" -H 'Content-Type: application/json' "${AUTH_HDR[@]}" \
  -d "{\"date\":\"2026-08-02\",\"description\":\"Opening\",\"postings\":[{\"account_id\":1,\"commodity\":\"CNY\",\"amount\":\"5000.00\"},{\"account_id\":$ID_EQUITY,\"commodity\":\"CNY\",\"amount\":\"-5000.00\"}]}" >/dev/null
echo "=== 设备A：记 meal 交易 (-38.50) ==="
curl -s -X POST "$B/api/transactions" -H 'Content-Type: application/json' "${AUTH_HDR[@]}" \
  -d "{\"date\":\"2026-08-03\",\"description\":\"Lunch\",\"postings\":[{\"account_id\":1,\"commodity\":\"CNY\",\"amount\":\"-38.50\"},{\"account_id\":2,\"commodity\":\"CNY\",\"amount\":\"38.50\"}]}" >/dev/null

# --- 设备B：拉取（增量，since=0）---
echo; echo "=== 设备B：pull since=0 ==="
PULL=$(curl -s "$B/api/sync/pull?client_id=devB&since=0" "${AUTH_HDR[@]}")
echo "$PULL" | python3 -c "import sys,json;d=json.load(sys.stdin);print('checkpoint=',d['checkpoint'],'changes=',len(d['changes']));[print(' ',c['entity_type'],c['entity'].get('name') or c['entity'].get('description')) for c in d['changes']]"
SEQ_B=$(echo "$PULL" | python3 -c "import sys,json;print(json.load(sys.stdin)['checkpoint'])")
echo "设备B 水位=$SEQ_B"

# --- 设备B：离线新建账户 + 交易（模拟断网，仅本地产生，之后推送）---
echo; echo "=== 设备B：离线创建 Liabilities:Card 并记一笔刷卡消费 ==="
UUID_CARD=$(python3 -c "import uuid;print('card-'+uuid.uuid4().hex[:8])")
UUID_TXN_B=$(python3 -c "import uuid;print('txn-'+uuid.uuid4().hex[:8])")
PUSH_B=$(curl -s -X POST "$B/api/sync/push" -H 'Content-Type: application/json' "${AUTH_HDR[@]}" -d "{
  \"client_id\":\"devB\",
  \"since\":$SEQ_B,
  \"changes\":[
    {\"entity_type\":\"account\",\"entity\":{\"uuid\":\"$UUID_CARD\",\"name\":\"Liabilities:Card\",\"type\":\"Liabilities\",\"open_date\":\"2026-08-01\",\"commodity_restriction\":\"CNY\"}},
    {\"entity_type\":\"transaction\",\"entity\":{\"uuid\":\"$UUID_TXN_B\",\"date\":\"2026-08-05\",\"description\":\"刷卡买菜\",\"postings\":[{\"account_uuid\":\"$UUID_CARD\",\"commodity\":\"CNY\",\"amount\":\"-200.00\"},{\"account_uuid\":\"$UUID_FOOD\",\"commodity\":\"CNY\",\"amount\":\"200.00\"}]}}
  ]
}")
echo "$PUSH_B" | python3 -c "import sys,json;d=json.load(sys.stdin);print('push 返回 checkpoint=',d['checkpoint'],'accepted=',d['accepted'])"

# --- 设备A：增量拉取设备B的变更 ---
echo; echo "=== 设备A：pull since=上次水位(此处用 0 模拟另一台新设备，应看到全部) ==="
PULL_A=$(curl -s "$B/api/sync/pull?client_id=devA&since=0" "${AUTH_HDR[@]}")
echo "$PULL_A" | python3 -c "import sys,json;d=json.load(sys.stdin);print('checkpoint=',d['checkpoint'],'changes=',len(d['changes']))"

# --- 幂等性：设备B 再次 push 同样内容，accepted 应为 0 ---
echo; echo "=== 幂等性：设备B 重复 push 同一批 ==="
PUSH_B2=$(curl -s -X POST "$B/api/sync/push" -H 'Content-Type: application/json' "${AUTH_HDR[@]}" -d "{
  \"client_id\":\"devB\",
  \"since\":$SEQ_B,
  \"changes\":[
    {\"entity_type\":\"account\",\"entity\":{\"uuid\":\"$UUID_CARD\",\"name\":\"Liabilities:Card\",\"type\":\"Liabilities\",\"open_date\":\"2026-08-01\",\"commodity_restriction\":\"CNY\"}},
    {\"entity_type\":\"transaction\",\"entity\":{\"uuid\":\"$UUID_TXN_B\",\"date\":\"2026-08-05\",\"description\":\"刷卡买菜\",\"postings\":[{\"account_uuid\":\"$UUID_CARD\",\"commodity\":\"CNY\",\"amount\":\"-200.00\"},{\"account_uuid\":\"$UUID_FOOD\",\"commodity\":\"CNY\",\"amount\":\"200.00\"}]}}
  ]
}")
echo "$PUSH_B2" | python3 -c "import sys,json;d=json.load(sys.stdin);print('重复 push accepted=',d['accepted'],'(应为 0)')"

# --- 余额与导出校验 ---
echo; echo "=== 账户余额 ==="
curl -s "$B/api/accounts" "${AUTH_HDR[@]}" | python3 -c "import sys,json;[print(a['name'],a.get('balances')) for a in json.load(sys.stdin)['accounts']]"
echo "=== 导出 + bean-check ==="
curl -s "$B/api/export" "${AUTH_HDR[@]}" -o data/sync_test.beancount
.venv/Scripts/bean-check.exe data/sync_test.beancount && echo "bean-check PASS (exit=$?)"

kill $SRV 2>/dev/null || true
rm -f ./.sync_test_bin 2>/dev/null || true   # 部分环境 rm 被安全删除拦截，忽略清理失败
echo; echo "DONE"