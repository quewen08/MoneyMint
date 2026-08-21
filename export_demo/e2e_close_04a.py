"""0.4-A 删除语义升级为 close 的端到端验证。

流程：注册 → 建账户 → 记一笔 → 关闭账户 → 验证（默认列表隐藏 / include_closed 含 close_date / 引用交易保留）→ 导出 → bean-check。
"""
import datetime
import json
import os
import subprocess
import sys
import time
import urllib.request

BASE = "http://127.0.0.1:8080"

# 唯一后缀，避免重复跑时用户名/账户名冲突
_TS = str(int(time.time()) % 100000)


def req(method, path, token=None, body=None):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(f"{BASE}{path}", data=data, headers=headers, method=method)
    with urllib.request.urlopen(r) as resp:
        raw = resp.read().decode()
        return resp.status, (json.loads(raw) if raw else {})


def main():
    # 1. 注册（首个用户建默认账本成 owner）；若已存在则登录
    try:
        _, reg = req("POST", "/api/auth/register",
                     body={"username": "alice04a", "password": "secret123", "display_name": "Alice"})
        token = reg["token"]
        print(f"[1] 注册成功，token={token[:16]}...")
    except urllib.error.HTTPError as e:
        if e.code != 409:
            raise
        _, reg = req("POST", "/api/auth/login",
                     body={"username": "alice04a", "password": "secret123"})
        token = reg["token"]
        print(f"[1] 已存在，登录成功，token={token[:16]}...")

    # 2. 建账户
    _, w = req("POST", "/api/accounts", token,
              body={"name": f"Assets:Wallet{_TS}:CNY", "type": "Assets", "open_date": "2026-08-01", "commodity": "CNY"})
    wallet_uuid = w["uuid"]
    wallet_id = w["id"]
    print(f"[2] 建钱包账户 uuid={wallet_uuid} id={wallet_id}")
    _, f = req("POST", "/api/accounts", token,
              body={"name": f"Expenses:TestFood{_TS}", "type": "Expenses", "open_date": "2026-08-01"})
    food_id = f["id"]
    print(f"[2] 建支出分类 id={food_id}")
    expected_name = f"Assets:Wallet{_TS}:CNY"

    # 3. 记一笔买菜
    _, t = req("POST", "/api/transactions", token, body={
        "date": "2026-08-05", "flag": "*", "description": "买菜",
        "postings": [
            {"account_id": wallet_id, "commodity": "CNY", "amount": "-5.00"},
            {"account_id": food_id, "commodity": "CNY", "amount": "5.00"},
        ],
    })
    print(f"[3] 记一笔 uuid={t['uuid']}")

    # 4. 关闭钱包账户
    _, clos = req("DELETE", f"/api/accounts/{wallet_uuid}", token)
    print(f"[4] 关闭账户: {clos}")

    # 5a. 默认列表不含已关闭
    _, lst = req("GET", "/api/accounts", token)
    open_names = [a["name"] for a in lst["accounts"]]
    assert expected_name not in open_names, f"已关闭账户不应出现在默认列表: {open_names}"
    print(f"[5a] 默认列表不含已关闭账户 ✓ ({len(open_names)} 个开放账户)")

    # 5b. include_closed=1 含已关闭 + close_date
    _, lstc = req("GET", "/api/accounts?include_closed=1", token)
    closed = [a for a in lstc["accounts"] if a["name"] == expected_name]
    if not closed:
        names_with_close = [(a["name"], a.get("close_date")) for a in lstc["accounts"] if a.get("close_date")]
        print(f"[debug] _TS={_TS} expected={expected_name} include_closed总数={len(lstc['accounts'])} 已关闭={names_with_close}")
    assert closed and closed[0].get("close_date"), f"已关闭账户应有 close_date: {closed}"
    print(f"[5b] include_closed 返回已关闭账户 close_date={closed[0]['close_date']} ✓")

    # 5c. 引用交易保留（检查有交易仍引用已关闭账户）
    _, txns = req("GET", "/api/transactions", token)
    refs_closed = [t for t in txns["transactions"]
                   if any(p.get("account") == expected_name for p in t["postings"])]
    assert refs_closed, "close 后引用该账户的交易应保留"
    print(f"[5c] close 后引用交易保留 ✓ (共 {len(txns['transactions'])} 笔，{len(refs_closed)} 笔引用已关闭账户)")

    # 6. close 后重建同名应失败（Beancount 不允许 close 后再 open 同名，账户名仍占用）
    try:
        reopen_date = (datetime.date.today() + datetime.timedelta(days=1)).isoformat()
        req("POST", "/api/accounts", token,
            body={"name": expected_name, "type": "Assets", "open_date": reopen_date, "commodity": "CNY"})
        raise AssertionError("close 后重建同名应失败（409），但实际成功")
    except urllib.error.HTTPError as e:
        assert e.code == 409, f"close 后重建同名应返回 409，实际 {e.code}"
        print(f"[6] close 后重建同名被拒绝（409 Conflict）✓")

    # 7. 导出 + bean-check
    r = urllib.request.Request(f"{BASE}/api/export", headers={"Authorization": f"Bearer {token}"})
    with urllib.request.urlopen(r) as resp:
        bean_text = resp.read().decode()
    export_path = f"export_demo/e2e_close_04a_{_TS}.bean"
    with open(export_path, "w", encoding="utf-8") as fh:
        fh.write(bean_text)
    print(f"[7] 导出 {len(bean_text)} 字节 → {export_path}")
    # 校验含 close 指令
    assert f"close Assets:Wallet{_TS}:CNY" in bean_text, "导出应含 close 指令"
    # 重建同名后应有两个 open + 一个 close（旧账户 close，新账户 open）
    print("    导出含 close 指令 ✓")

    # bean-check
    res = subprocess.run(
        [".venv/Scripts/python.exe", "-m", "beancount.scripts.check", export_path],
        capture_output=True, text=True,
    )
    print(f"[8] bean-check 退出码={res.returncode}")
    if res.returncode != 0:
        print("    STDOUT:", res.stdout)
        print("    STDERR:", res.stderr)
        sys.exit(1)
    print("    bean-check 通过 ✓")

    print("\n=== 0.4-A 端到端验证全部通过 ===")


if __name__ == "__main__":
    main()
