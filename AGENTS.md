# AGENTS.md — 家庭协作记账系统（类 Beancount 云记账）

> 项目给 AI 助手的一致上下文。会话开始前先读。
> 用户：全栈独做（老板），后端 Go / 前端 Flutter / 设计稿均本人拍板。沟通可直接给结论。
> 提醒：因独做全栈，"分配负责人/邀请角色关注"类提醒不适用，改为提醒其自写验收标准、留档、关键模块自评审。

---

## 1. 项目概述

跑在 NAS 个人云上的**复式记账应用**，对标 Beancount 数据模型：
- 前端：Flutter Web（PWA），浏览器访问，无原生客户端计划。
- 后端：Go 标准库 + `modernc.org/sqlite`（纯 Go，免 CGO，便于 NAS arm64 交叉编译）。
- 数据库：SQLite（开发期）；后期平滑切 Postgres（DB 访问层已抽象）。
- 家庭成员**共享同一账本**；支持离线多端记账 + 增量同步。

**核心不变约束**：导出的 `.beancount` 必须能被 `bean-check` 直接校验通过（退出码 0、无报错）。

**当前阶段**：P0 / P1-A（前端打磨+离线联调）/ P1-B（删除语义/逐条 push/认证增强/成员管理）/ P1-C（多账本隔离+角色鉴权+账本设置+多文件目录导出）均已完成验证。下一阶段：上线就绪（HTTPS 部署、真机双端联调）。

---

## 2. 技术栈与本机环境

| 工具 | 版本 / 路径 | 说明 |
|---|---|---|
| Go | 1.26.5，`/d/Environment/go/bin` | Git Bash 需 `export PATH="$PATH:/d/Environment/go/bin"` |
| Flutter | 3.35.1，`/d/Environment/flutter/bin` | 同上需加 PATH |
| Python | 项目内 `.venv` | 仅用于 `bean-check`（`beancount` 包），不污染系统 |
| GOPROXY | `https://goproxy.cn,direct` | `proxy.golang.org` 国内不可达 |
| PUB_HOSTED_URL | `https://pub.flutter-io.cn` | 前端 `flutter pub get` 国内镜像 |

验证用（已建好可跳过）：
```bash
python3 -m venv .venv
.venv/Scripts/python.exe -m pip install beancount
```

---

## 3. 目录结构

```
.
├── schema.sql            # 初始核心表 DDL（含 Beancount 映射注释）
├── cmd/server/main.go    # 后端入口
├── internal/             # 后端分层：config → domain → db → repository → service → handler → server
│   ├── db/{db,migrate}.go   # 开库 + schema 迁移；迁移框架见 §4.2
│   ├── repository/       # 全部 SQL 集中于此（account/transaction/member/sync/session/user/ledger）
│   ├── service/          # 业务：校验/事务编排/余额/自然键合并/同步协议/成员权限
│   ├── handler/          # 薄 HTTP 适配层
│   ├── server/           # 组合根：路由 + CORS/auth 中间件 + 依赖装配
│   ├── auth/auth.go      # bcrypt / token / context 用户态（纯函数）
│   └── export/{beancount,files}.go  # 单文件 + 多文件目录导出
├── frontend/lib/         # Flutter Web：MVVM（models/api/store/sync + viewmodels + screens + widgets + app）
├── export_demo/          # Python/sh 验证参考
├── data/                 # 运行时 SQLite（git 忽略）
└── docs/                 # design-spec.md / sync-design.md / prototype/index.html
```

**前端架构要点**：MVVM 分层 + 响应式双布局（≥1024 走 PC 侧边栏，<1024 降级移动底栏）。
- 根 `AppScope`（`app/scope.dart`，`InheritedWidget`）注入 `LedgerApi`/`LocalStore` 与两个 controller。
- 本地真相源 `core/store/local_store.dart`（sembast+IndexedDB）；同步编排 `core/sync/sync_service.dart`。
- **坑（务必遵守）**：页面读取 `AppScope.of(context)` 必须放在 `build`/`didChangeDependencies`，**禁止**放 `initState`（会触发 `dependOnInheritedWidgetOfExactType called before initState completed` 红屏）。

---

## 4. 数据库 Schema

金额一律 `NUMERIC`；应用层以**定点十进制字符串**或最小货币单位整数写入，**禁止浮点**。
同一 transaction 下，按 commodity 分组的 `postings.amount` 之和必须为 0（有符号数，可负）。

核心表（`schema.sql`，幂等 `IF NOT EXISTS`）：
- `users`(id, username UNIQUE, password_hash, display_name, status)
- `ledgers`(id, name, owner_id, default_commodity)
- `ledger_members`(ledger_id, user_id, role[owner|editor|viewer]) — 权限在此层
- `accounts`(id, ledger_id, uuid, name, type[5类], open_date, close_date, commodity_restriction, deleted_at)
- `commodities`(id, ledger_id, uuid, symbol, name, precision)
- `prices`(id, ledger_id, uuid, commodity, currency, date, rate, source)
- `transactions`(id, ledger_id, uuid UNIQUE, date, flag[*|!], description, version, created_by, deleted_at)
- `postings`(id, transaction_id, account_id, commodity, amount 有符号, position)

**迁移（`internal/db/migrate.go`，追加式、幂等、记录 `schema_migrations`）**：
- 001_sync：补 `accounts/commodities/prices` 的 `uuid`+`updated_at`；建 `sync_log`(id 自增即全局水位, ledger_id, entity_type, entity_uuid, op, created_at)；重建 `sync_checkpoints`(ledger_id, client_id, last_seq)；建 `sessions`。
- 002_delete_semantics：交易/账户加 `deleted_at`（软删）；账户重建部分唯一索引 `(ledger_id,name) WHERE deleted_at IS NULL`；旧库 owner 密码置为 `owner123456`。

> **修改 schema 必须走迁移脚本**，不要直接改 `schema.sql` 后手动 ALTER 线上库。

---

## 5. 认证与权限模型

家庭账本是**共享账本**：未登录 401 不可见 / 账本成员可见共享数据 / 非成员 403。
- 密码 bcrypt 加盐；登录签发 32 字节 token（存 `sessions`，有效期 30 天），退出即删、旧 token 立即失效；`POST /api/auth/refresh` 换新 token（旧立即失效）。
- 注册语义：首个注册用户创建默认账本并成为 owner；后续注册仅建账号，等待 owner 按用户名添加为成员（业务端点对未加入者返回 403）。
- 成员管理仅 owner 可操作（列表/添加/改角色/移除/重置密码——重置使该用户全部会话失效）。角色：`owner | editor | viewer`。
- `authMiddleware` 包裹所有业务路由；放行 `/api/health` 与 register/login；自服务端点仅需登录态，业务端点额外校验为默认账本成员。

---

## 6. 离线同步协议

**append-only + 不可变 + 水位增量**（天然几乎无冲突）。
- 不可变交易 + uuid 身份键：改账 = 新增冲正交易，绝不覆盖；删除 = 软删 + `op=delete`。
- `sync_log` 全局水位：每次接受 create/delete 追加一行，`id` 单调即服务端水位。
- pull：`GET /api/sync/pull?since=<水位>&client_id=<设备>` 取回 `id>since` 的全部变更（delete 仅 `{uuid}`）。
- push：`POST /api/sync/push` 逐条提交，单条失败不阻塞其余；响应 `{checkpoint, accepted, results:[{index, entity_type, uuid, status, accepted, server_uuid, error}]}`（按请求顺序）。先 create（commodity→account→transaction）再 delete（transaction→account）。
- 自然键合并：推送账户时若同账本存在未删同名账户，保留先到者，`server_uuid` 回传真实 uuid，前端据此重映射本地 `loc-` uuid。
- 前端离线优先：`local_store` 为本地真相源，断网可记账；联网先 pull 合并再 push 本地队列，失败保留重试。

**关键陷阱（勿混用）**：
| 路径 | posting 账户字段 | 取值 |
|---|---|---|
| 在线 `POST /api/transactions` | `account_id` | 服务端自增整数 id |
| 离线 `POST /api/sync/push` | `account_uuid` | 字符串 uuid；所有 posting 必须带，不能用账户名 |

---

## 7. API 端点清单

| 端点 | 方法 | 鉴权 | 说明 |
|---|---|---|---|
| `/api/health` | GET | 公开 | `{"status":"ok"}` |
| `/api/auth/register` | POST | 公开 | 首个用户建默认账本成 owner，后续仅建账号待邀请，返回 token |
| `/api/auth/login` | POST | 公开 | 登录，返回 token |
| `/api/auth/logout` | POST | 登录 | 注销当前会话 |
| `/api/auth/me` | GET | 登录 | 当前用户（role 恒空串） |
| `/api/auth/refresh` | POST | 登录 | 换发新 token（旧立即失效） |
| `/api/auth/change-password` | POST | 登录 | 改密后该用户全部会话失效并签发新 token |
| `/api/ledgers` | GET/POST | 登录 | 列出所属账本（含角色）/ 建新账本（创建者成 owner） |
| `/api/ledgers/{id}` | PATCH | 登录+**owner** | 账本改名 / 默认币种 |
| `/api/accounts` | GET/POST | 登录+成员 | 账户列表（含余额）/ 新建（生成 uuid + appendSyncLog） |
| `/api/accounts/{uuid}` | DELETE | 登录+成员 | 软删账户（连带引用交易，写 sync_log delete）；幂等 404 |
| `/api/transactions` | GET/POST | 登录+成员 | 流水 / 记一笔（postings 用 `account_id`；按币种借贷平衡校验） |
| `/api/transactions/{uuid}` | DELETE | 登录+成员 | 软删交易；幂等 404 |
| `/api/ledger/members` | GET/POST | 登录+**owner** | 成员列表 / 按用户名添加已注册用户 |
| `/api/ledger/members/{userID}` | POST/DELETE | 登录+**owner** | 改角色 / 移除（禁移除自己或 owner） |
| `/api/ledger/members/{userID}/reset-password` | POST | 登录+**owner** | 重置密码并使该用户全部会话失效 |
| `/api/export` | GET | 登录+成员 | 导出单文件 `.beancount` |
| `/api/export/archive` | GET | 登录+成员 | 导出多文件目录 zip（main.bean + accounts/ + date/） |
| `/api/sync/pull` | GET | 登录+成员 | 增量拉取（since + client_id） |
| `/api/sync/push` | POST | 登录+成员 | 逐条幂等提交离线变更 |

> 多账本上下文（P1-C）：业务端点需带 `X-Ledger-Id` 头指定账本；缺失回退默认账本 id=1。`viewer` 只读（非 GET 一律 403）、`editor` 可写、`owner` 可管理。
> CORS：`Origin: *`，Headers 含 `Content-Type, Authorization, X-Ledger-Id`，Methods 含 `GET,POST,DELETE,PATCH,OPTIONS`。

---

## 8. 约定与惯例（给 AI 的硬约束）

- **金额**：永远用定点十进制字符串或整数最小单位；**禁止**浮点参与金额运算/存储。导出按 commodity precision 格式化。
- **Beancount 兼容**：导出文本必须能过 `bean-check`。改 `internal/export/beancount.go` / `files.go` 后必须重新跑 bean-check。
- **迁移优先**：任何 schema 改动走 `migrate.go`，幂等、记录 `schema_migrations`。
- **借贷平衡**：后端 `handleCreateTransaction` 按币种平衡校验；前端记一笔页实时指示。
- **UUID 引用**：同步实体一律用 `uuid` 互引，勿用服务端自增 `id` 跨端传递；注意在线用 `account_id`、离线 push 用 `account_uuid`（§6）。
- **不可变**：交易一旦创建不原地改；改账 = 新增冲正交易。
- **注释语言**：代码注释用中文；保留 `schema.sql` 中 Beancount 映射注释。

---

## 9. 已知边界 / 下期

- 跨币种兑换（cost 注解）：P0 单币种，需 `postings` 加 `cost_commodity`/`cost_amount`。
- 商品/价格（`commodities`/`prices`）尚无 CRUD 端点，仅表 + 导出引用；`screens/commodities/` 占位。
- 前端下载用 `dart:html`，WASM 目标告警（JS 目标正常）。
- 真机双端联调（浏览器 A/B 同时在线、离线多设备端到端）尚未在浏览器侧确认（后端与模拟脚本已验证）。
- 下期：Beancount 导入、OFX/CSV 自动导入、规则引擎、小票 OCR、商品/价格 CRUD、Postgres 横向扩展。

---

## 10. 快速运行与测试

```bash
# 后端
export PATH="$PATH:/d/Environment/go/bin"
LEDGER_DB=data/ledger.db go run ./cmd/server        # 默认 :8080，LISTEN=:9000 可覆盖

# 注册拿 token
TOKEN=$(curl -s -X POST http://127.0.0.1:8080/api/auth/register -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"secret123","display_name":"Alice"}' | sed 's/.*"token":"\([^"]*\)".*/\1/')
AUTH="Authorization: Bearer $TOKEN"

# 建账户 / 记一笔（postings 用 account_id）/ 导出校验
curl -s -X POST http://127.0.0.1:8080/api/accounts -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"Assets:Cash:CNY","type":"Assets","open_date":"2026-08-01","commodity":"CNY"}'
curl -s -X POST http://127.0.0.1:8080/api/transactions -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"date":"2026-08-02","description":"早餐","flag":"*","postings":[{"account_id":1,"commodity":"CNY","amount":"-10.00"},{"account_id":2,"commodity":"CNY","amount":"10.00"}]}'
curl -s http://127.0.0.1:8080/api/export -H "$AUTH" -o data/family.beancount
.venv/Scripts/python.exe -m beancount.scripts.check data/family.beancount   # 退出码 0 通过

# 前端
cd frontend && export PATH="$PATH:/d/Environment/flutter/bin"
export PUB_HOSTED_URL="https://pub.flutter-io.cn"
flutter pub get && flutter run -d chrome

# 部署 NAS（纯 Go 免 CGO）
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o familyledger ./cmd/server
```

> 本机 `bean-check.exe` 直接运行静默返回 1（Windows 包装脚本问题），改用 `.venv/Scripts/python.exe -m beancount.scripts.check` 校验。
> 建议用 Nginx/Caddy 反代加 HTTPS；前端部署把 `api.dart` 的 `http://localhost:8080` 改为 NAS 地址。
