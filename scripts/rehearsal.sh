#!/usr/bin/env bash
# 迁移预演：把线上库用 VACUUM INTO 复制到 data/rehearsal/，在临时端口连续启动
# 服务两次（迁移执行两遍），断言迁移结果与幂等性，并对副本库导出 .beancount 做
# bean-check 校验。
#
# 硬约束：
#   - 只读源库（data/ledger.db），绝不写入、绝不迁移线上库；
#   - 所有临时库/日志/构建缓存都在项目目录内（data/rehearsal、.cache）；
#   - 不安装任何全局依赖。
#
# 用法：
#   bash scripts/rehearsal.sh
#   REHEARSAL_SOURCE_DB=/path/to/copy.db bash scripts/rehearsal.sh
#   REHEARSAL_SKIP_BEAN_CHECK=1 bash scripts/rehearsal.sh   # 仅限本机无校验器时
#
# 退出码：0 全部断言通过；非 0 任一断言失败。

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

SOURCE_DB="${REHEARSAL_SOURCE_DB:-$ROOT_DIR/data/ledger.db}"
REHEARSAL_DIR="$ROOT_DIR/data/rehearsal"
DB="$REHEARSAL_DIR/ledger.db"
LOG_DIR="$REHEARSAL_DIR/logs"
EXPORT_DIR="$REHEARSAL_DIR/export"
METRICS1="$REHEARSAL_DIR/metrics_run1.txt"
METRICS2="$REHEARSAL_DIR/metrics_run2.txt"
SERVER_LOG1="$LOG_DIR/run1.log"
SERVER_LOG2="$LOG_DIR/run2.log"

mkdir -p "$REHEARSAL_DIR" "$LOG_DIR" "$EXPORT_DIR" "$ROOT_DIR/.cache/tmp"
export GOCACHE="${GOCACHE:-$ROOT_DIR/.cache/go-build}"
export GOTMPDIR="${GOTMPDIR:-$ROOT_DIR/.cache/tmp}"
if ! command -v go >/dev/null 2>&1 && [[ -x /d/Environment/go/bin/go ]]; then
  export PATH="$PATH:/d/Environment/go/bin"
fi
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"

# ---- Python（项目内 venv 优先；仅用于 sqlite 快照/断言与 bean-check） ----
PYTHON_BIN="${PYTHON_BIN:-}"
if [[ -z "$PYTHON_BIN" ]]; then
  for cand in \
    "$ROOT_DIR/.venv/Scripts/python.exe" \
    "$ROOT_DIR/.venv/bin/python" \
    "python3" \
    "python"; do
    if [[ "$cand" == */* || "$cand" == *\\* ]]; then
      [[ -x "$cand" ]] || continue
    else
      command -v "$cand" >/dev/null 2>&1 || continue
    fi
    if "$cand" -c 'import sqlite3' >/dev/null 2>&1; then
      PYTHON_BIN="$cand"
      break
    fi
  done
fi
if [[ -z "$PYTHON_BIN" ]]; then
  echo "❌ 未找到可用的 Python（需带 sqlite3 模块）" >&2
  exit 1
fi

SERVER_PID=""
cleanup() {
  if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" 2>/dev/null; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

source_fingerprint() {
  "$PYTHON_BIN" - "$SOURCE_DB" <<'PY'
import hashlib, os, sys
p = sys.argv[1]
if not os.path.exists(p):
    print("missing")
    sys.exit(0)
h = hashlib.sha256()
with open(p, "rb") as f:
    for chunk in iter(lambda: f.read(1024 * 1024), b""):
        h.update(chunk)
st = os.stat(p)
print(f"{h.hexdigest()} {st.st_size} {st.st_mtime_ns}")
PY
}

# ---- 1) VACUUM INTO 一致性快照（只读源库，目标在 data/rehearsal/） ----
rm -f "$DB" "$DB-wal" "$DB-shm"
rm -rf "$EXPORT_DIR"
mkdir -p "$EXPORT_DIR"

SOURCE_BEFORE="$(source_fingerprint)"
if [[ -f "$SOURCE_DB" ]]; then
  echo "📦 复制源库 → data/rehearsal/（VACUUM INTO，只读源库）…"
  "$PYTHON_BIN" - "$SOURCE_DB" "$DB" <<'PY'
import os, sqlite3, sys
src, dst = sys.argv[1], sys.argv[2]
if os.path.exists(dst):
    os.remove(dst)
conn = sqlite3.connect(src, timeout=30)
try:
    conn.execute("VACUUM INTO ?", (dst,))
finally:
    conn.close()
# 快照完整性自检
snap = sqlite3.connect(dst)
try:
    row = snap.execute("PRAGMA integrity_check").fetchone()
    if not row or row[0] != "ok":
        raise SystemExit("VACUUM INTO 快照 integrity_check 失败: %r" % (row,))
finally:
    snap.close()
print("✅ 快照完整性检查通过")
PY
else
  echo "ℹ️ 源库不存在（$SOURCE_DB），使用空库预演（仅验证迁移可执行与幂等）"
fi

# ---- 2) 构建服务二进制（缓存全部留在项目 .cache） ----
GOEXE="$(go env GOEXE)"
SERVER_BIN="$REHEARSAL_DIR/server${GOEXE}"
echo "🔨 构建预演服务（$SERVER_BIN）…"
go build -o "$SERVER_BIN" ./cmd/server

# ---- 3) 临时端口 + 两次启动（每次启动即执行迁移） ----
PORT="$("$PYTHON_BIN" - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
)"

wait_health() {
  "$PYTHON_BIN" - "$PORT" <<'PY'
import sys, time, urllib.request
port = int(sys.argv[1])
url = f"http://127.0.0.1:{port}/api/health"
for _ in range(300):
    try:
        with urllib.request.urlopen(url, timeout=0.2) as r:
            if r.status == 200:
                sys.exit(0)
    except Exception:
        time.sleep(0.1)
sys.exit(1)
PY
}

start_and_stop_server() {
  local log="$1"
  : > "$log"
  LEDGER_DB="$DB" \
  LISTEN="127.0.0.1:$PORT" \
  LEDGER_SCHEMA="$ROOT_DIR/schema.sql" \
  LEDGER_WEB_ROOT="$REHEARSAL_DIR/no-web" \
  LEDGER_BACKUP_ENABLED=false \
  LEDGER_REGISTRATION_ENABLED=false \
  "$SERVER_BIN" >"$log" 2>&1 &
  SERVER_PID=$!
  if ! wait_health; then
    echo "❌ 服务未就绪，日志：$log" >&2
    sed -n '1,120p' "$log" >&2 || true
    exit 1
  fi
  cleanup
  SERVER_PID=""
}

server_round_1() {
  start_and_stop_server "$SERVER_LOG1"
}
server_round_2() {
  start_and_stop_server "$SERVER_LOG2"
}

echo "🚀 第一次启动（执行迁移 012）…"
server_round_1

# ---- 4) 断言 + 指标落盘 ----
"$PYTHON_BIN" - "$DB" "$SOURCE_DB" >"$METRICS1" <<'PY'
import os, sqlite3, sys
db_path, src_path = sys.argv[1], sys.argv[2]
conn = sqlite3.connect(db_path)
try:
    def one(q, args=()):
        return conn.execute(q, args).fetchone()

    m012 = one("SELECT count(*) FROM schema_migrations WHERE name='012_opening_balance'")[0]
    if m012 != 1:
        raise SystemExit(f"012 迁移记录期望 1，实际 {m012}")

    ledgers = one("SELECT count(*) FROM ledgers")[0]
    if os.path.exists(src_path):
        src = sqlite3.connect(src_path)
        try:
            src_ledgers = src.execute("SELECT count(*) FROM ledgers").fetchone()[0]
        finally:
            src.close()
        if ledgers != src_ledgers:
            raise SystemExit(f"ledgers 行数与源库不一致：副本 {ledgers} / 源 {src_ledgers}")

    empty = one("SELECT count(*) FROM ledgers WHERE opening_date IS NULL OR opening_date=''")[0]
    if empty != 0:
        raise SystemExit(f"仍有 {empty} 个账本 opening_date 为空")

    for idx in ("idx_txn_opening_key", "idx_txn_opening_active", "idx_txn_reversal_of"):
        c = one("SELECT count(*) FROM sqlite_master WHERE type='index' AND name=?", (idx,))[0]
        if c != 1:
            raise SystemExit(f"缺少索引 {idx}")

    dups = one("""
        SELECT count(*) FROM (
          SELECT ledger_id, opening_account_uuid
          FROM transactions
          WHERE opening_account_uuid IS NOT NULL AND opening_account_uuid <> ''
            AND reversal_of IS NULL AND reversed_by IS NULL AND deleted_at IS NULL
          GROUP BY ledger_id, opening_account_uuid
          HAVING count(*) > 1
        )
    """)[0]
    if dups != 0:
        raise SystemExit(f"存在 {dups} 个账户仍有重复有效期初")

    print(f"migration_012={m012}")
    print(f"ledgers={ledgers}")
    print(f"empty_opening_date={empty}")
    print(f"active_duplicates={dups}")
    print(f"migration_log={one('SELECT count(*) FROM migration_log')[0]}")
    print(f"sync_log={one('SELECT count(*) FROM sync_log')[0]}")
    print(f"transactions={one('SELECT count(*) FROM transactions')[0]}")
finally:
    conn.close()
PY
echo "✅ 第一次迁移断言通过"

echo "🚀 第二次启动（重复执行迁移，验证幂等）…"
server_round_2

"$PYTHON_BIN" - "$DB" >"$METRICS2" <<'PY'
import sqlite3, sys
conn = sqlite3.connect(sys.argv[1])
try:
    def one(q):
        return conn.execute(q).fetchone()[0]

    m012 = one("SELECT count(*) FROM schema_migrations WHERE name='012_opening_balance'")
    ledgers = one("SELECT count(*) FROM ledgers")
    empty = one("SELECT count(*) FROM ledgers WHERE opening_date IS NULL OR opening_date=''")
    dups = one("""
        SELECT count(*) FROM (
          SELECT ledger_id, opening_account_uuid
          FROM transactions
          WHERE opening_account_uuid IS NOT NULL AND opening_account_uuid <> ''
            AND reversal_of IS NULL AND reversed_by IS NULL AND deleted_at IS NULL
          GROUP BY ledger_id, opening_account_uuid
          HAVING count(*) > 1
        )
    """)
    migration_log = one("SELECT count(*) FROM migration_log")
    sync_log = one("SELECT count(*) FROM sync_log")
    transactions = one("SELECT count(*) FROM transactions")

    print(f"migration_012={m012}")
    print(f"ledgers={ledgers}")
    print(f"empty_opening_date={empty}")
    print(f"active_duplicates={dups}")
    print(f"migration_log={migration_log}")
    print(f"sync_log={sync_log}")
    print(f"transactions={transactions}")
finally:
    conn.close()
PY

if ! diff -u "$METRICS1" "$METRICS2" >/dev/null; then
  echo "❌ 二次执行出现数据变化（应完全幂等）：" >&2
  diff -u "$METRICS1" "$METRICS2" >&2 || true
  exit 1
fi
echo "✅ 二次执行幂等（migration_log / sync_log / 交易数均未增长）"

# ---- 5) 源库必须零改动 ----
SOURCE_AFTER="$(source_fingerprint)"
if [[ "$SOURCE_BEFORE" != "$SOURCE_AFTER" ]]; then
  echo "❌ 源库指纹发生变化（预演不应写入线上库）" >&2
  echo "   before: $SOURCE_BEFORE" >&2
  echo "   after : $SOURCE_AFTER" >&2
  exit 1
fi
echo "✅ 源库未被修改（sha256/size/mtime 不变）"

# ---- 6) 副本库导出 .beancount 并过 bean-check ----
if [[ "${REHEARSAL_SKIP_BEAN_CHECK:-}" == "1" ]]; then
  echo "ℹ️ REHEARSAL_SKIP_BEAN_CHECK=1，跳过导出校验"
  echo ""
  echo "✅ 预演全部通过（副本库：$DB）"
  exit 0
fi

EXPORT_GO="$REHEARSAL_DIR/export_check.go"
cat >"$EXPORT_GO" <<'GO'
// 预演辅助程序：从副本库逐账本导出 .beancount。仅在 data/rehearsal/ 下临时生成。
package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"familyledger/internal/export"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "用法: export_check <db> <outdir>")
		os.Exit(1)
	}
	dbPath, outDir := os.Args[1], os.Args[2]
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "创建导出目录失败: %v\n", err)
		os.Exit(1)
	}
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开副本库失败: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	rows, err := conn.Query(`SELECT id FROM ledgers ORDER BY id`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "查询账本失败: %v\n", err)
		os.Exit(1)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			fmt.Fprintf(os.Stderr, "读取账本 id 失败: %v\n", err)
			os.Exit(1)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "遍历账本失败: %v\n", err)
		os.Exit(1)
	}

	for _, id := range ids {
		text, err := export.ExportLedger(conn, id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "导出账本 %d 失败: %v\n", id, err)
			os.Exit(1)
		}
		path := filepath.Join(outDir, fmt.Sprintf("ledger_%d.beancount", id))
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "写 %s 失败: %v\n", path, err)
			os.Exit(1)
		}
		fmt.Printf("导出 %s\n", path)
	}
}
GO

go run "$EXPORT_GO" "$DB" "$EXPORT_DIR"

shopt -s nullglob
exports=("$EXPORT_DIR"/*.beancount)
if [[ ${#exports[@]} -eq 0 ]]; then
  echo "ℹ️ 副本库没有账本，跳过 bean-check"
else
  for f in "${exports[@]}"; do
    echo "🔍 bean-check: $f"
    if ! "$PYTHON_BIN" -m beancount.scripts.check "$f" </dev/null; then
      echo "❌ bean-check 未通过：$f" >&2
      exit 1
    fi
  done
fi

echo ""
echo "✅ 预演全部通过（副本库：$DB）"
