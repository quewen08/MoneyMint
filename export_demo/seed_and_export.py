#!/usr/bin/env python3
"""
家庭协作记账 - Beancount 导出器 (验证参考实现)
读取 schema.sql 定义的 SQLite 库，导出合法 .beancount 文本。
逻辑与 internal/export/beancount.go 完全一致，仅用于在本机用 bean-check 验证格式。
"""
import sqlite3, os, sys
from decimal import Decimal, ROUND_HALF_UP, getcontext
getcontext().prec = 40

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SCHEMA = os.path.join(ROOT, "schema.sql")
DBPATH = os.path.join(ROOT, "export_demo", "ledger.db")
OUT = os.path.join(ROOT, "export_demo", "sample.beancount")


def init_db():
    if os.path.exists(DBPATH):
        os.remove(DBPATH)
    con = sqlite3.connect(DBPATH)
    con.execute("PRAGMA foreign_keys=ON")
    with open(SCHEMA, encoding="utf-8") as f:
        con.executescript(f.read())
    return con


def seed(con):
    c = con.cursor()
    c.executemany("INSERT INTO users(username,password_hash,display_name) VALUES (?,?,?)",
                  [("alice", "x", "Alice"), ("bob", "x", "Bob"), ("carol", "x", "Carol")])
    c.execute("INSERT INTO ledgers(name,owner_id,default_commodity) VALUES (?,?,?)", ("Smith Family", 1, "CNY"))
    c.executemany("INSERT INTO ledger_members(ledger_id,user_id,role,invited_by) VALUES (1,?,?,1)",
                  [(1, "owner"), (2, "editor"), (3, "viewer")])
    c.executemany("INSERT INTO commodities(ledger_id,symbol,name,precision) VALUES (1,?,?,?)",
                  [("CNY", "人民币", 2), ("USD", "美元", 2), ("BTC", "比特币", 8)])
    accs = [
        (1, "Assets:Cash:CNY", "Assets", "2024-01-01", None, None),
        (1, "Assets:Bank:CNY", "Assets", "2024-01-01", None, None),
        (1, "Assets:Foreign:USD", "Assets", "2024-01-01", None, None),
        (1, "Assets:Crypto:BTC", "Assets", "2024-02-01", None, "BTC"),
        (1, "Expenses:Food:CNY", "Expenses", "2024-01-01", None, None),
        (1, "Expenses:Travel:USD", "Expenses", "2024-01-01", None, None),
        (1, "Income:Salary:CNY", "Income", "2024-01-01", None, None),
        (1, "Liabilities:CreditCard:CNY", "Liabilities", "2024-01-01", None, None),
        (1, "Equity:Opening:Balances", "Equity", "2024-01-01", None, None),
    ]
    c.executemany("INSERT INTO accounts(ledger_id,name,type,open_date,close_date,commodity_restriction) VALUES (?,?,?,?,?,?)", accs)
    c.executemany("INSERT INTO prices(ledger_id,commodity,currency,date,rate,source) VALUES (1,?,?,?,?,?)",
                  [("BTC", "CNY", "2024-02-01", str(Decimal("300000.00")), "manual"),
                   ("BTC", "CNY", "2024-03-01", str(Decimal("350000.00")), "manual"),
                   ("USD", "CNY", "2024-01-15", str(Decimal("7.20")), "manual")])

    def add_txn(uuid, date, flag, desc, postings):
        c.execute("INSERT INTO transactions(ledger_id,uuid,date,flag,description,created_by) VALUES (1,?,?,?,?,1)",
                  (uuid, date, flag, desc))
        tid = c.lastrowid
        for i, (acc, commodity, amount) in enumerate(postings):
            c.execute("INSERT INTO postings(transaction_id,account_id,commodity,amount,position) VALUES (?,?,?,?,?)",
                      (tid, acc, commodity, str(Decimal(amount)), i))

    # T1 期初余额
    add_txn("txn-0001", "2024-01-02", "*", "Opening balance",
            [(9, "CNY", "-5000.00"), (1, "CNY", "5000.00")])
    # T2 工资
    add_txn("txn-0002", "2024-01-10", "*", "Monthly salary",
            [(2, "CNY", "12000.00"), (7, "CNY", "-12000.00")])
    # T3 买菜
    add_txn("txn-0003", "2024-01-15", "*", "Groceries",
            [(5, "CNY", "320.50"), (1, "CNY", "-320.50")])
    # T4 还信用卡
    add_txn("txn-0004", "2024-01-20", "*", "CC payment",
            [(8, "CNY", "1000.00"), (2, "CNY", "-1000.00")])
    # T5 美元差旅 (单币种 USD)
    add_txn("txn-0005", "2024-03-02", "*", "Trip to US",
            [(3, "USD", "-100.00"), (6, "USD", "100.00")])
    con.commit()


def fmt_amount(amount, precision):
    q = Decimal(1).scaleb(-precision)
    s = Decimal(amount).quantize(q, rounding=ROUND_HALF_UP)
    return format(s, "f")


def esc(s):
    return str(s).replace("\\", "\\\\").replace('"', '\\"')


def export(con, ledger_id):
    c = con.cursor()
    commodities = {}
    for sym, name, prec in c.execute("SELECT symbol,name,precision FROM commodities WHERE ledger_id=?", (ledger_id,)):
        commodities[sym] = (name, prec)

    accounts = list(c.execute(
        "SELECT name,type,open_date,close_date,commodity_restriction FROM accounts WHERE ledger_id=? ORDER BY open_date,name",
        (ledger_id,)))

    prices = list(c.execute(
        "SELECT commodity,currency,date,rate FROM prices WHERE ledger_id=? ORDER BY date", (ledger_id,)))

    txns = []
    for tid, uuid, date, flag, desc in c.execute(
            "SELECT id,uuid,date,flag,description FROM transactions WHERE ledger_id=? ORDER BY date,id", (ledger_id,)):
        posts = [(an, com, amt, pos) for an, com, amt, pos in c.execute(
            "SELECT a.name,p.commodity,p.amount,p.position FROM postings p JOIN accounts a ON a.id=p.account_id "
            "WHERE p.transaction_id=? ORDER BY p.position", (tid,))]
        txns.append((date, flag, desc, posts))

    # 收集所有出现过的币种
    used = set(commodities.keys())
    for name, type_, od, cd, restr in accounts:
        if restr:
            used.add(restr)
    for com, cur, d, r in prices:
        used.add(com); used.add(cur)
    for d, fl, de, posts in txns:
        for an, com, amt, pos in posts:
            used.add(com)

    # 用于 commodity 指令的日期：所有日期的最小值（保证在一切使用之前声明）
    dates = [od for _, _, od, _, _ in accounts] + [d for _, _, d, _ in prices] + [d for d, _, _, _ in txns]
    epoch = min(dates) if dates else "2000-01-01"

    prec_of = lambda sym: commodities.get(sym, (None, 2))[1]

    L = []
    title = c.execute("SELECT name FROM ledgers WHERE id=?", (ledger_id,)).fetchone()[0]
    L.append(f'; -*- mode: beancount -*-\n; Exported from ledger "{esc(title)}"\n')
    L.append(f'option "title" "{esc(title)}"\n\n')

    # 1) commodity 指令（按符号排序）
    for sym in sorted(used):
        name, prec = commodities.get(sym, (None, 2))
        L.append(f"{epoch} commodity {sym}\n")
        if name:
            L.append(f'  name: "{esc(name)}"\n')
        L.append(f'  precision: "{prec}"\n')
    L.append("\n")

    # 2) open / close 指令
    for name, type_, od, cd, restr in accounts:
        if restr:
            L.append(f"{od} open {name} {restr}\n")
        else:
            L.append(f"{od} open {name}\n")
    L.append("\n")
    for name, type_, od, cd, restr in accounts:
        if cd:
            L.append(f"{cd} close {name}\n")
    if any(cd for *_, cd in [(a[3] for a in accounts)]):
        L.append("\n")

    # 3) price 指令
    for com, cur, d, r in prices:
        L.append(f"{d} price {com} {fmt_amount(r, prec_of(cur))} {cur}\n")
    L.append("\n")

    # 4) 交易
    for d, fl, de, posts in txns:
        L.append(f'{d} {fl} "{esc(de)}"\n')
        for an, com, amt, pos in posts:
            L.append(f"  {an} {fmt_amount(amt, prec_of(com))} {com}\n")
        L.append("\n")

    return "".join(L)


def main():
    con = init_db()
    seed(con)
    text = export(con, 1)
    con.close()
    with open(OUT, "w", encoding="utf-8") as f:
        f.write(text)
    sys.stdout.write(text)


if __name__ == "__main__":
    main()
