# #4 离线多端同步协议设计稿

> 范围：家庭多人 + 多端离线记账的同步方案。
> 状态：**已确认并实现**（以下三项待决已拍板，代码已落地并冒烟通过）。

## 1. 核心思路：append-only + 不可变 + 水位增量

复式记账的天然优势：**记账 = 只追加交易，从不原地改写**。
- 改账 = 新增一笔冲正/调整交易，原交易保留。
- 删除账户/价格 = 发一条「墓碑」(tombstone)，不物理删除。
- 因此每条记录一旦创建就**不可变**，靠全局唯一 `uuid` 标识。

同步的本质就退化成：**各端把自己本地新增的、对方没有的记录，推给对方；再把对方新增的拉回来。**
由于记录彼此独立且不可变，**几乎无冲突**（唯一的真实冲突在「配置实体按自然键重复创建」，见 §5）。

## 2. 同步单元与身份

| 概念 | 说明 |
|---|---|
| 同步单元 | 账本（`ledgers.id`）。家庭共享同一账本即同步同一份数据。 |
| 记录身份 | `uuid`（客户端生成，全局唯一）。`transactions` 已有；`accounts`/`commodities`/`prices` 需补。 |
| 水位 | 服务端自增单调序列 `server_seq`（见 `sync_log`）。各客户端记录「我已同步到哪个 seq」。 |
| 客户端身份 | `client_id`（UUID，存 IndexedDB/localStorage）。每浏览器/设备一个。 |

## 3. 服务端新增：`sync_log` 事件日志（统一水位）

不按表分别记水位，而是用一张事件日志统一承载所有实体的变更顺序：

```sql
CREATE TABLE sync_log (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    ledger_id    INTEGER NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    entity_type  TEXT NOT NULL,          -- 'transaction' | 'account' | 'commodity' | 'price'
    entity_uuid  TEXT NOT NULL,          -- 对应记录的 uuid
    op           TEXT NOT NULL,          -- 'create' | 'delete'
    server_seq   INTEGER NOT NULL UNIQUE,-- 全局单调水位(由 AUTOINCREMENT 保证)
    created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_synclog_ledger_seq ON sync_log(ledger_id, server_seq);
```

- 任何「创建」都追加一行 `op='create'`；任何「删除/关闭」追加 `op='delete'`。
- 服务端自己的 API（建账户、记交易、删价格…）**也必须写 sync_log**，这样 A 端直连服务端产生的变更，B 端 pull 时也能拿到。
- `server_seq` 即统一水位，pull 用 `WHERE server_seq > ?` 增量取。

## 4. API 设计

### 4.1 推送（客户端 → 服务端）
```
POST /api/sync/push
{ "ledger_id": 1, "client_id": "<uuid>",
  "entities": [
    {"type":"transaction","uuid":"...","payload":{...}},
    {"type":"account","uuid":"...","payload":{...}}
  ]
}
```
服务端：
1. 按 `uuid` 去重（`INSERT ... ON CONFLICT(uuid) DO NOTHING`）。
2. 配置实体按自然键去重（见 §5）。
3. 对每个新接受记录，追加 `sync_log`（拿到 `server_seq`）。
4. 返回 `{ "max_seq": N, "accepted":[uuid...], "rejected":[uuid...] }`。

### 4.2 拉取（服务端 → 客户端）
```
GET /api/sync/pull?ledger_id=1&client_id=<uuid>&since=<seq>
```
返回：
```json
{ "max_seq": N,
  "changes": [
    {"seq":12,"type":"transaction","op":"create","uuid":"...","payload":{...}},
    {"seq":13,"type":"price","op":"delete","uuid":"..."}
  ]
}
```
客户端按 `uuid` upsert/删除本地副本，把自身水位更新为 `max_seq`。

### 4.3 水位存储（改造 `sync_checkpoints`）
```sql
-- 原表是 (ledger_id, user_id, last_txn_id)；改为按客户端：
CREATE TABLE sync_checkpoints (
    ledger_id      INTEGER NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    client_id      TEXT NOT NULL,
    last_seq       INTEGER NOT NULL DEFAULT 0,   -- 已同步到的 server_seq
    last_synced_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (ledger_id, client_id)
);
```
pull 时若不传 `since`，默认用 checkpoint 的 `last_seq`；处理完写回。

## 5. 冲突处理（唯一真实冲突点）

| 场景 | 处理 |
|---|---|
| 两客户端各自新增**不同 uuid 的交易** | 都保留（append-only，天然无冲突） |
| 两客户端离线各自新增**同名账户**（不同 uuid） | 按自然键 `(ledger_id, name)` 合并：服务端保留先到者，返回其 uuid，后到客户端的本地记录改为指向已有 uuid |
| 同一价格被两侧重复添加 | 同上，按 `(ledger_id, commodity, currency, date)` 自然键合并 |
| 交易删除 | 不允许物理删；需要「撤销」= 新增冲正交易（保留不可变约束） |
| 账户/价格删除 | 发 tombstone，pull 时本地标记删除 |

→ 结论：**交易零冲突；配置实体靠自然键合并，冲突可解且罕见。**

## 6. 客户端离线（Flutter Web PWA + IndexedDB）

- 本地用 **IndexedDB** 镜像一份账本（表结构同服务端，主键 `uuid`），存 `client_id`、`last_seq`、待推送队列。
- 离线记账：写本地 IndexedDB（生成 `uuid`，标 `pending`），UI 立即可见。
- 联网瞬间：① push 本地 `pending` 实体 → ② pull `since=last_seq` → ③ 应用 changes、更新 `last_seq`、清 `pending`。
- **PWA**：加 `manifest.webmanifest` + service worker，缓存应用外壳，断网也能打开页面（数据来自 IndexedDB）。
- 余额/报表由本地 IndexedDB 实时算，不依赖网络。

## 7. 决策记录（已确认 / 已实现）

> 三项原待决项已由老板拍板，对应代码已落地，2026-08-12 冒烟验证通过。

1. **同步范围（已确认：全实体 synced + sync_log 统一水位）** ✅
   - 采用本稿方案：accounts / commodities / prices / transactions 全部纳入 `sync_log`，统一水位 `sync_log.id`。
   - 已实现：迁移 001 补 `uuid`+`updated_at` 并建 `sync_log`、重建 `sync_checkpoints`；`/api/sync/pull` 用 `WHERE id > since` 增量拉，`/api/sync/push` 按 `uuid` 去重写入并 append `sync_log`。

2. **认证前置（已确认：最小登录 + token 鉴权）** ✅
   - 已实现 `internal/auth` + `/api/auth/{register,login,logout,me}`；`authMiddleware` 统一鉴权：health/auth 端点公开，其余 401（未登录）/ 403（非账本成员）。
   - 注册自动加入默认账本（editor）；sync 端点同样受 token + 账本成员资格保护。
   - 验证：未登录 pull/push/accounts 均 401；登录→访问 200；错误密码 401；退出后旧 token 401。

3. **schema 变更（已同意）** ✅
   - 已实现迁移 001（幂等，开发期旧库 ALTER 补列 + 回填 uuid）。`accounts`/`commodities`/`prices` 现带 `uuid` 唯一索引与 `updated_at`；`sync_log`、`sync_checkpoints`、`sessions` 已建。

## 8. 实施步骤（确认后）

1. 迁移脚本：`uuid`/`updated_at` 补列 + `sync_log` + 新 `sync_checkpoints`。
2. 写 `sync_log` 的写入钩子挂到现有 create/delete 路径。
3. 实现 `/api/sync/push` 与 `/api/sync/pull`。
4. 后端自测：双客户端（两个 client_id）互相 push/pull，验证增量与无冲突。
5. Flutter 端：IndexedDB 本地层 + 离线记账 + 联网 reconcile + PWA 外壳。
6. 端到端：断网记 3 笔 → 联网同步 → 另一客户端 pull 到 → bean-check 仍通过。
