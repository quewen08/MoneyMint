# FamBean PC 端优化方案（v1）

> 基于 ezBookkeeping 等成熟产品的 PC 端参考图整理，结合 FamBean 的 Beancount 复式账芯约束。
> 决策日期：2026-08-17。核心原则：**UI 可借鉴，账芯不复式记账模型不动**。

---

## 0. 已确认的设计决策

| # | 决策 | 结论 |
|---|---|---|
| 1 | 分类实体 | **不拆独立实体**。分类 = `Expenses`/`Income` 类型账户，三级结构：`Expenses:食品饮料:食品`。仅维护 `Expenses` 的二级分类，三级系统内置+用户添加。 |
| 2 | 记一笔模式 | **统一方案 A**：默认弹窗「简化模式」（支出/收入/转账），保留「高级」双录入口。 |
| 3 | 账户视觉 | **先做图标 + 颜色**（账户/分类显示 icon 与色块）。 |
| 4 | 标签 | **本期做**。标签挂到交易上，导出用 Beancount `#tag` 语法。 |
| 5 | 实施顺序 | 见 §6。 |

---

## 1. 设计语言基线（沿用既有主题）

- 主色 `--blue #3B6EF6`、成功 `--green #2BA471`、警示 `--warn #E8590C`。
- 背景 `--bg #F5F6F8`、卡片 `--card #FFFFFF`、文字 `--ink #1A1D1F`、次要 `--sub #6B7280`、分割线 `--line #E5E7EB`。
- PC 侧边栏底色 `--sidebar #15203B`（保持现有 desktop_shell 深色侧栏）。
- 圆角 12–16；金额一律等宽数字 `font-variant-numeric: tabular-nums`。

---

## 2. 全局布局

```
┌────────────┬───────────────────────────────────────────┐
│ 侧边栏      │  顶栏：账本名 · 同步徽标 · [记一笔] · 用户   │
│ 家庭记账    ├───────────────────────────────────────────┤
│ · 概览      │                                           │
│ · 交易流水  │            内容区（按路由切换）              │
│ · 账户管理  │                                           │
│ · 成员管理  │                                           │
│ · 商品与价格│                                           │
│ · 导入与导出│                                           │
│ · 报表分析  │                                           │
│ · 同步管理  │                                           │
│ · 设置      │                                           │
│             │                                           │
│ [记一笔]    │                                           │
└────────────┴───────────────────────────────────────────┘
```

侧边栏分组与现有 `navigation.dart` 的 `pcSidebar` 一致（概览/管理/工具/系统），新增「分类」「标签」归入管理分组。

---

## 3. 各页面改造方案

### 3.1 总览（原 仪表盘）
- 指标卡网格：净资产 / 总资产 / 总负债 / 本月支出 / 本月收入。
- 收支趋势图（近 12 个月）折线或柱状。
- 右侧/底部「最近交易」列表（取最近 5 笔）。
- 纯本地聚合，无接口变更。

### 3.2 交易流水
- 顶部筛选栏：交易类型（全部/支出/收入/转账）、时间范围（本月/上月/本年/自定义）、分类、账户、标签、关键词。
- 双 Tab：**交易列表**（表格，可分页/滚动）、**交易日历**（按日聚合）。
- 交易类型推导（核心规则，见附录 A）。
- 工具栏：添加（弹窗）/ 导入（跳转）/ 刷新（同步）。
- 纯本地，无接口变更。

### 3.3 记一笔（弹窗，简化模式）
- 居中弹窗，Tab：支出 / 收入 / 转账。
- 支出：金额 + 分类（= Expenses 二级账户，带 icon）+ 账户（= Assets/Liabilities，带余额）+ 日期 + 标签 + 描述。
- 收入：金额 + 分类（= Income 二级账户）+ 账户 + 日期 + 标签 + 描述。
- 转账：金额 + 来源账户 + 目标账户 + 日期 + 标签 + 描述。
- 底部「高级」按钮 → 展开双录分录表单（现有 record_screen）。
- 提交时底层生成标准复式 posting（规则见附录 B）。
- 无接口变更（最终仍走 `createTransaction`）。

### 3.4 账户管理
- 左侧：净资产/总资产/总负债摘要 + 类型树（可展开子账户）。
- 右侧：账户卡片（icon、名称、币种、余额、最近交易）。
- 点击账户 → 账户详情（流水 + 对账单，复用 TransactionsScreen 过滤）。
- 新增「新建账户」弹窗（见 3.6）。

### 3.5 报表分析
- 左栏维度切换：分类分析 / 趋势分析 / 资产趋势。
- 分类分析：环形图（按分类账户汇总支出）。
- 趋势分析：月度收入/支出/净流入柱状。
- 资产趋势：净资产/总资产/总负债月末余额走势。
- 顶部时间范围（本年/去年/全部）。
- 纯本地聚合，无接口变更。

### 3.6 新建账户（弹窗）
- 字段：账户分类（资产/负债/权益/收入/支出）、账户类型（如借记/信用卡/虚拟…作为子类型或父账户）、**图标选择器**、**颜色选择器**、币种（默认 CNY）、期初余额。
- 期初余额提交时自动生成：`Equity:OpeningBalances → 账户` 初始交易。
- 需后端账户表加字段（见 §5）。

### 3.7 分类管理（新增页面）
- 复用 `Expenses`/`Income` 类型账户，二级为「分类」。
- 支持三级（系统内置 + 用户添加），每个分类设 icon + color。
- 编辑/删除/隐藏（隐藏 = 软禁用于新建选择，不删历史）。

### 3.8 标签（新增页面）
- 标签列表（带使用计数）。
- 新建交易时输入标签，自动补全。
- 需后端交易表加 `tags`（见 §5）。

---

## 4. 前端文件改动清单

| 文件 | 动作 | 说明 |
|---|---|---|
| `screens/dashboard/dashboard_screen.dart` | 改 | 卡片网格 + 趋势图 |
| `screens/transactions/transactions_screen.dart` | 改 | 筛选栏 + 双 Tab + 类型推导 |
| `screens/transactions/transactions_filter_bar.dart` | 新增 | 筛选组件 |
| `screens/record/record_dialog.dart` | 新增 | 简化模式弹窗 |
| `screens/record/record_screen.dart` | 保留 | 作为高级双录入口服 |
| `screens/accounts/accounts_screen.dart` | 改 | 摘要 + 类型树 + 卡片 |
| `screens/accounts/account_detail_screen.dart` | 新增 | 账户流水/对账单 |
| `screens/accounts/account_dialog.dart` | 新增 | 新建账户弹窗（icon/color/期初） |
| `screens/reports/reports_screen.dart` | 改 | 图表化 |
| `screens/categories/categories_screen.dart` | 新增 | 分类管理 |
| `screens/tags/tags_screen.dart` | 新增 | 标签管理 |
| `widgets/charts/*` | 新增 | 折线/柱状/环形图（推荐 `fl_chart`） |
| `widgets/account_icon.dart` | 新增 | 图标 + 颜色渲染 |
| `core/models/account.dart` | 改 | 增 `icon`/`color`/`parentUuid`/`subType` |
| `core/models/transaction.dart` | 改 | 增 `tags: List<String>` |
| `app/navigation.dart` | 改 | 新增分类/标签导航项 |

---

## 5. 后端 API / 数据库变更

> **不新增独立 REST 资源**。所有数据仍走本地 store + 同步协议。只扩展现有 schema 与同步实体字段。

### 5.1 `accounts` 表扩展
```sql
ALTER TABLE accounts ADD COLUMN icon TEXT;          -- 图标标识
ALTER TABLE accounts ADD COLUMN color TEXT;         -- 颜色 hex
ALTER TABLE accounts ADD COLUMN parent_uuid TEXT;    -- 父账户（三级结构用）
ALTER TABLE accounts ADD COLUMN sub_type TEXT;       -- 账户子类型
```
- 走迁移脚本 `internal/db/migrate.go`（003_account_visual）。
- 同步 push/pull 的 account 实体需携带上述字段。

### 5.2 `transactions` 表扩展
```sql
ALTER TABLE transactions ADD COLUMN tags TEXT;  -- JSON 数组，如 ["餐饮","出差"]
```
- 走迁移脚本 `004_txn_tags`。
- 同步 transaction 实体与在线 `POST /api/transactions` 请求体加 `tags`。
- Beancount 导出（`internal/export/beancount.go`）：交易行尾追加 `#tag1 #tag2`。

### 5.3 不做的接口
- 暂不做服务端统计接口（保持离线可用），所有报表基于本地聚合。
- 期初余额不需要新接口，由前端生成初始交易。

---

## 6. 实施顺序（已确认）

1. 账户 `icon/color/parent_uuid` + 分类页（为交易 UI 打底）
2. 新建账户 / 新建交易 弹窗（最高频操作）
3. 总览页卡片化 + 趋势图
4. 交易流水筛选 + 日历视图
5. 统计分析页图表化
6. 标签页

---

## 附录 A：交易类型推导规则

| 条件（posting 账户类型） | 显示类型 |
|---|---|
| 含 `Assets`/`Liabilities` 减少 + 含 `Expenses` 增加 | 支出 |
| 含 `Income` 减少 + 含 `Assets`/`Liabilities` 增加 | 收入 |
| 两端均为 `Assets` 或均为 `Liabilities` | 转账 |
| 其他 | 其他 |

## 附录 B：简化模式 posting 生成规则

- 支出 X：`Assets/负债账户 -X` + `Expenses:分类 +X`
- 收入 X：`Income:分类 -X` + `Assets/负债账户 +X`
- 转账 X：`来源账户 -X` + `目标账户 +X`

> 所有金额按 commodity 分组的借贷和必须为 0，复用现有 `createTransaction` 平衡校验。
