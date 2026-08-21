"""临时验证脚本：注册→列出账户→推送带标签的交易→导出→bean-check。"""
import json
import os
import subprocess
import urllib.request
import urllib.error

BASE = "http://127.0.0.1:9123"
DB = os.path.join(os.path.dirname(__file__), "..", "data", "verify_tags.db")


def req(method, path, token=None, body=None):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(r) as resp:
            return resp.status, json.loads(resp.read().decode() or "{}")
    except urllib.error.HTTPError as e:
        return e.code, json.loads(e.read().decode() or "{}")


def main():
    # 注册（首个用户 = 默认账本 owner，seed 默认账户）
    st, j = req("POST", "/api/auth/register", body={"username": "verify", "password": "secret123"})
    assert st in (200, 201), (st, j)
    token = j["token"]
    print("registered, token len", len(token))

    # 列出账户，找到 Assets:Cash 与 Expenses:Food 的 uuid
    st, j = req("GET", "/api/accounts", token=token)
    assert st == 200, (st, j)
    by_name = {a["name"]: a["uuid"] for a in j["accounts"]}
    cash = by_name.get("Assets:Cash")
    food = by_name.get("Expenses:Food")
    assert cash and food, by_name
    print("cash", cash, "food", food)

    # 推送带标签的交易（沿用同步协议）
    txn = {
        "uuid": "verify-txn-new",
        "date": "2026-08-10",
        "flag": "*",
        "description": "午餐",
        "tags": ["餐饮", "出差"],
        "postings": [
            {"account_uuid": cash, "commodity": "CNY", "amount": "-38.50"},
            {"account_uuid": food, "commodity": "CNY", "amount": "38.50"},
        ],
    }
    st, j = req("POST", "/api/sync/push", token=token, body={
        "client_id": "verify-dev", "since": 0,
        "changes": [{"entity_type": "transaction", "op": "create", "entity": txn}],
    })
    assert st == 200, (st, j)
    print("push results", j["results"])

    # 导出
    st, text = (lambda r: (r.status, r.read().decode()))(
        urllib.request.urlopen(urllib.request.Request(
            BASE + "/api/export", headers={"Authorization": "Bearer " + token}))
    )
    assert st == 200
    out = os.path.join(os.path.dirname(__file__), "verify_tags.bean")
    with open(out, "w", encoding="utf-8") as f:
        f.write(text)
    print("exported bytes", len(text))

    # bean-check
    r = subprocess.run([os.path.join(os.path.dirname(__file__), "..", ".venv", "Scripts", "bean-check.exe"), out],
                       capture_output=True, text=True)
    print("bean-check returncode", r.returncode)
    print(r.stdout)
    print(r.stderr)
    if r.returncode != 0:
        raise SystemExit("bean-check FAILED")
    # 断言标签语法出现在导出中
    assert "#餐饮" in text and "#出差" in text, "tags missing in export"
    print("TAGS_EXPORT_OK")


if __name__ == "__main__":
    main()
