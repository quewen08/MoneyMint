# export_demo — 验证参考目录（非运行依赖）

本目录是一份**独立的验证参考**，用于在本机直观验证两件事：

1. **Beancount 导出格式正确性**：数据库 → `.beancount` 文本 → `bean-check` 通过。
2. **离线多端同步协议**：双设备（client_id）互相 push/pull、增量水位、幂等。

> 应用本身**不依赖**本目录。真正的导出在 `internal/export/beancount.go`、同步在
> `internal/handler` / `internal/service`，都由 Go 实现。这里只是「用另一种语言 / 脚本把同样的逻辑跑一遍」以便肉眼核对与本地演示。
> 删除本目录不影响 app 运行，也不影响 `bean-check`（后者用项目根 `.venv`）。

---

## 1. `seed_and_export.py` — 导出格式验证

读 `schema.sql` 建库 → 灌一份家庭样例数据（多币种账户、价格、5 笔交易）→
导出 `sample.beancount`，逻辑与 `internal/export/beancount.go` 完全一致。

**前置**：项目根已建 `.venv` 并装了 `beancount`（见根目录 README「快速开始」）。

```bash
# 在项目根目录执行
.venv/Scripts/python.exe export_demo/seed_and_export.py
```

- 自动重建 `export_demo/ledger.db`（样例库）与 `export_demo/sample.beancount`（导出文本）。
- 文本同时打印到 stdout。
- 校验：`.venv/Scripts/bean-check.exe export_demo/sample.beancount` → 退出码 0 即通过。

> 该脚本只依赖 `schema.sql` 的核心表，不建立 `sync_log`/`sessions`（导出用不到），属正常。

---

## 2. `sync_test.sh` — 双端同步模拟（Go 后端真实验证）

这是**唯一会真跑 Go 后端 HTTP 服务**的验证脚本，覆盖 `#4` 同步协议端到端：

```
设备A 在线建账户+记账
   → 设备B pull 合并（since=0 增量拉取）
   → 设备B 离线新建账户+交易（交易 posting 用 account_uuid），之后 push 回服务端
   → 设备A 再 pull 合并
   → 设备B 重复 push 同一批（验证幂等 accepted=0）
   → 打印余额 + 导出 + bean-check
```

**关键字段约定（与实测一致）**：

- push 请求信封为 `{client_id, since, changes:[{entity_type, entity}]}`，**不是** `{"accounts":[...]}`。
- 交易实体 posting 引用 `account_uuid`（字符串），且**所有** posting 都必须带，不能用账户 `name`。
- 账户实体字段：`{uuid, name, type, open_date, commodity_restriction}`。

**前置**：
- `go` 在 PATH（脚本内已 `export PATH="$PATH:/d/Environment/go/bin"`；本机 Go 装在非默认路径时按需改）。
- 项目根已建 `.venv`（`bean-check` 用）。

```bash
bash export_demo/sync_test.sh
```

脚本会随机选一个空闲端口、编译并后台启动服务、跑完自动 `kill`。过程中会生成临时库
`data/sync_test_<时间戳>.db` 与编译产物 `.sync_test_bin`，两者均已加入 `.gitignore`，
若系统安全删除策略拦截 `rm` 无法自动清除，可手动清理。

> 脚本已内置认证：开头 `register`/`login` 拿到 token，所有业务请求带
> `Authorization` 头（`AUTH_HDR` 数组避免分词把 token 拆成多余参数导致 401）。
> `seed_and_export.py` 不受影响（它直连 SQLite，不碰 HTTP）。

---

## 3. `smoke_p1b.sh` — P1-B 后端冒烟（迁移 002 / 成员 / 删除 / 逐条 push / 认证增强）

P1-B（2026-08-13）全链路冒烟验证脚本，覆盖 21 项断言：

```
① 注册语义    首个用户=owner；后续用户仅建账号；未受邀访问业务端点 403；自服务端点 200
② 成员管理    仅 owner 可列表/添加/改角色（editor→viewer）
③ 删除语义    DELETE /api/transactions/{uuid} 软删 + sync_log delete；重复删除幂等 404
④ 逐条 push   同名账户自然键合并回传 server_uuid；引用缺失单条 error 不阻塞其余
⑤ 认证增强    change-password 旧 token 失效；refresh 换新 token；owner 重置成员密码
⑥ 导出        bean-check 退出码 0
```

```bash
# 启动全新实例（冒烟脚本要求空库；端口自选）
LEDGER_DB=data/p1b_verify.db LISTEN=:8123 go run ./cmd/server &
B=http://127.0.0.1:8123 bash export_demo/smoke_p1b.sh   # PASS=21 FAIL=0
```

> 脚本对全新库运行（注册语义依赖「首个用户」判定）；跑完的库为生成物，可清理。

---

## 4. 验证记录

- **2026-08-12**：`sync_test.sh` 双端模拟通过（余额正确、bean-check PASS）。
- **2026-08-13**：在另一轮独立端到端手测中（全新测试库、`:8099`）再次确认：
  注册→建账户→在线记账（`account_id`）→借贷平衡校验→导出 `bean-check` 退出码 0；
  `sync/pull`、`sync/push`（账户/交易，`account_uuid`）幂等（重复 accepted=0）。详见 `AGENTS.md` §9.2。
- **2026-08-13**：`smoke_p1b.sh` 全链路冒烟 **21/21 通过**（全新 `data/p1b_verify.db`、`:8123`）——
  迁移 002 / 注册语义 / 成员管理 / 删除语义 / 逐条 push + 自然键合并 / 认证增强 / bean-check。详见 `AGENTS.md` §9.4。

---

## 文件清单

| 文件 | 作用 | 是否生成物（可删/会重建） |
|---|---|---|
| `seed_and_export.py` | 建样例库并导出，验证 bean-check | 源码，保留 |
| `sync_test.sh` | 双端同步模拟（Go 后端真实验证） | 源码，保留 |
| `smoke_p1b.sh` | P1-B 冒烟（迁移 002 / 成员 / 删除 / 逐条 push / 认证增强） | 源码，保留 |
| `ledger.db` | `seed_and_export.py` 生成的样例库 | 生成物（已加入 .gitignore） |
| `sample.beancount` | `seed_and_export.py` 生成的导出文本 | 生成物（已加入 .gitignore） |
