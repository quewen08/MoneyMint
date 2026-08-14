-- ============================================================
-- 家庭协作记账系统 — P0 数据库 Schema (SQLite)
-- ------------------------------------------------------------
-- 设计目标：
--   1. 支持多位家庭成员在同一账本(ledger)内协作记账
--   2. 数据可序列化为合法 Beancount 文本并通过 bean-check
--   3. DB 访问层做抽象，后期可平滑切换 Postgres
--
-- 金额说明：
--   postings.amount / prices.rate 使用 NUMERIC。
--   应用层务必以「最小货币单位整数」或 decimal 字符串写入，
--   避免浮点误差；导出时按 commodity 精度格式化。
--
-- 平衡校验：
--   同一 transaction 下，按 commodity 分组的 amount 之和必须为 0。
--   amount 为有符号数（可负），导出时按符号输出正负。
--
-- 幂等：所有 CREATE 均带 IF NOT EXISTS，服务重启/重复执行均安全。
-- 本文件只定义「初始核心表」；sync_log / 新 sync_checkpoints 等
-- 由 migrations/ 下的迁移脚本创建（见 internal/db/migrate.go）。
-- ============================================================

PRAGMA foreign_keys = ON;

-- ------------------------------------------------------------
-- 用户（自建账号，配置/邀请新增）
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name  TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'active',   -- active | disabled
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

-- ------------------------------------------------------------
-- 账本（家庭组）：共享记账的基本单元
-- 一个账本 = 一个家庭组的账本；成员通过 ledger_members 关联
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ledgers (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    name              TEXT NOT NULL,
    owner_id          INTEGER NOT NULL REFERENCES users(id),
    default_commodity TEXT,                           -- 默认记账币种(可选)
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at        TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (owner_id, name)                            -- 同一拥有者下账本名唯一
);

-- ------------------------------------------------------------
-- 账本成员与角色（家庭关系表；权限在此层控制）
--   role: owner  | editor | viewer
--   owner  : 管理成员/权限、删除账本
--   editor : 增删改账目
--   viewer : 只读
-- 邀请：由现有成员生成邀请(配置或邀请码)，accepted 后写入本表
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ledger_members (
    ledger_id   INTEGER NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    user_id     INTEGER NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
    role        TEXT NOT NULL DEFAULT 'viewer',         -- owner | editor | viewer
    invited_by  INTEGER REFERENCES users(id),
    joined_at   TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (ledger_id, user_id),
    CHECK (role IN ('owner','editor','viewer'))
);

-- ------------------------------------------------------------
-- 账户（Beancount: open 指令的实体）
--   type: Assets | Liabilities | Equity | Income | Expenses
--   open_date  -> open 指令日期（必须早于该账户的首笔交易）
--   close_date -> close 指令日期（可空）
--   commodity_restriction -> open 指令的币种限定参数(可选)
--   uuid       -> 全局唯一，离线同步身份键（迁移 001 补齐）
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS accounts (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    ledger_id  INTEGER NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    uuid       TEXT,                                   -- 迁移 001 补齐 UNIQUE 索引
    name       TEXT NOT NULL,                          -- Beancount 账户名(ASCII), 如 Assets:Cash:CNY
    display_name TEXT,                                 -- 中文显示名(可选), 如 现金; 导出仍用 name
    type       TEXT NOT NULL,                          -- 5 类账户之一
    open_date  TEXT NOT NULL,                          -- 'YYYY-MM-DD'
    close_date TEXT,                                   -- 'YYYY-MM-DD' 可空
    commodity_restriction TEXT,                        -- 限定币种(可选)
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (ledger_id, name),
    CHECK (type IN ('Assets','Liabilities','Equity','Income','Expenses'))
);

-- ------------------------------------------------------------
-- 币种 / 商品（Beancount: commodity 指令）
--   uuid -> 同步身份键（迁移 001 补齐）
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS commodities (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ledger_id   INTEGER NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    uuid        TEXT,
    symbol      TEXT NOT NULL,                          -- 如 CNY / USD / BTC
    name        TEXT,                                   -- 显示名
    precision   INTEGER NOT NULL DEFAULT 2,             -- 小数位
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (ledger_id, symbol)
);

-- ------------------------------------------------------------
-- 价格（Beancount: price 指令）
--   commodity 以 currency 计价，在 date 日单价为 rate
--   uuid -> 同步身份键（迁移 001 补齐）
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS prices (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ledger_id   INTEGER NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    uuid        TEXT,
    commodity   TEXT NOT NULL,                           -- 商品
    currency    TEXT NOT NULL,                           -- 计价币种
    date        TEXT NOT NULL,                           -- 'YYYY-MM-DD'
    rate        NUMERIC NOT NULL,                        -- 单价
    source      TEXT,                                    -- 来源(手动/导入)
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_prices_ledger_date ON prices(ledger_id, date);

-- ------------------------------------------------------------
-- 交易（Beancount: transaction 指令）
--   交易整体不可变；改账 = 新增冲正/调整交易。
--   uuid   : 全局唯一，作为离线同步的身份键
--   version: 同步版本号（仅用于元数据排序/冲突检测）
--   flag   : '*' 普通 | '!' 待核对
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS transactions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ledger_id   INTEGER NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    uuid        TEXT NOT NULL UNIQUE,                    -- 全局唯一, 同步身份
    date        TEXT NOT NULL,                           -- 'YYYY-MM-DD'
    flag        TEXT NOT NULL DEFAULT '*',               -- * | !
    description TEXT NOT NULL DEFAULT '',
    version     INTEGER NOT NULL DEFAULT 1,              -- 同步版本号
    created_by  INTEGER REFERENCES users(id),
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_txn_ledger_date ON transactions(ledger_id, date);

-- ------------------------------------------------------------
-- 分录（Beancount: postings）
--   借贷平衡: 同一 transaction 下, 按 commodity 分组的 amount 之和 = 0
--   amount 为有符号数(可负); 导出时按符号输出
--   position: 保证导出顺序稳定
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS postings (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    transaction_id INTEGER NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    account_id    INTEGER NOT NULL REFERENCES accounts(id),
    commodity     TEXT NOT NULL,                         -- 该笔记账币种
    amount        NUMERIC NOT NULL,                      -- 有符号金额
    position      INTEGER NOT NULL DEFAULT 0             -- 输出顺序
);
CREATE INDEX IF NOT EXISTS idx_postings_txn ON postings(transaction_id);
CREATE INDEX IF NOT EXISTS idx_postings_account ON postings(account_id);

-- ------------------------------------------------------------
-- 同步水位（离线多端增量同步）—— 旧结构由迁移 001 重建为
--   (ledger_id, client_id, last_seq, last_synced_at)。
-- 此处不再定义，避免与迁移脚本冲突。
-- ------------------------------------------------------------
