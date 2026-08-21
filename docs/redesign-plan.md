# FamBean 全端优化方案（v2）

> 基于 ezBookkeeping 等成熟产品的 PC + 移动端参考图整理，结合 FamBean 的 Beancount 复式账芯约束。
> 决策日期：2026-08-17/18。核心原则：**UI 可借鉴，账芯不复式记账模型不动**。

---

## 0. 已确认的设计决策

| # | 决策 | 结论 |
|---|---|---|
| 1 | 分类实体 | **不拆独立实体**。分类 = `Expenses`/`Income` 类型账户，三级结构：`Expenses:食品饮料:食品`。仅维护 `Expenses` 的二级分类，三级系统内置+用户添加。 |
| 2 | 记一笔模式 | **统一方案 A**：默认弹窗/全屏页「简化模式」（支出/收入/转账），保留「高级」双录入口。底层仍生成标准复式 posting。 |
| 3 | 账户视觉 | **先做图标 + 颜色**（账户/分类显示 icon 与色块）。 |
| 4 | 标签 | **本期做**。标签挂到交易上，导出用 Beancount `#tag` 语法。 |
| 5 | 移动端导航 | 底部改为 **账单 / 资产 / 更多**（参考图），替代现有「首页 / 流水 / 我的」。 |
| 6 | 账单首页 | 默认 **日历视图**，可切换为列表视图；列表视图顶部加当月收支趋势图。 |
| 7 | 预算 | **本期不做**（参考图虽有预算模块，但需独立预算模型和预警逻辑，建议二期）。 |
| 8 | 账户字段 | 新增 `icon`/`color`/`parent_uuid`/`sub_type`；本期不做「计入总资产/记账时可选」开关（可二期加）。 |

---

## 1. 设计语言基线（沿用既有主题）

- 主色 `--blue #3B6EF6`、成功 `--green #2BA471`、警示 `--warn #E8590C`。
- 背景 `--bg #F5F6F8`、卡片 `--card #FFFFFF`、文字 `--ink #1A1D1F`、次要 `--sub #6B7280`、分割线 `--line #E5E7EB`。
- PC 侧边栏底色 `--sidebar #15203B`（保持现有 desktop_shell 深色侧栏）。
- 移动端以白色卡片、浅灰背景、圆角 12–16、大字号金额为主。
- 金额一律等宽数字 `font-variant-numeric: tabular-nums`。

---

## 2. PC 端布局（不变，见 v1）

左侧深色侧边栏 + 右侧内容区。侧边栏分组：概览（总览/交易流水）、管理（账户/分类/标签/成员/商品）、工具（导入导出/报表/同步）、系统（设置）。

---

## 3. 移动端布局

```
┌─────────────────────────────┐
│  状态栏 / 顶部标题            │
├─────────────────────────────┤
│                             │
│         内容区               │
│                             │
├─────────────────────────────┤
│  账单  │  资产  │  更多      │
└─────────────────────────────┘
```

底部 Tab：
- **账单**：日历首页 / 交易列表 / 统计。
- **资产**：净资产、总资产、总负债、账户分组。
- **更多**：设置、成员、同步、导入导出、报表（PC 平铺功能的收纳入口）。

> 原有 `mobileTabs`（首页/流水/我的）废弃，改为此三 Tab。PC 端仍保持 9 模块侧边栏。

---

## 4. 移动端各页面改造方案

### 4.1 账单首页（默认日历视图）
- 顶部：账本名下拉、当前月份选择器、搜索、统计入口。
- 主体：月历，有支出的日期下方显示金额（如 `-2.00`），今日高亮。
- 底部区：
  - 预算（占位/二期）。
  - 快捷记录：最近使用的分类 icon 行，点击直接记一笔并预填分类。
- 中央悬浮 + 按钮：记一笔。

### 4.2 账单列表视图
- 顶部月份选择器 + 搜索 + 统计。
- 切换控件：支出 / 收入（二选一），下方显示当月累计金额。
- 日度柱状图：当月 1–31 日每日支出/收入金额。
- 交易列表：按日期分组，每项显示分类 icon、分类名、账户、金额。
- 分类 icon 点击可二次筛选。

### 4.3 统计页（从账单进入）
- 顶部月份选择器。
- 指标卡网格：收入 / 支出 / 结余 / 日均消费 / 记账笔数（参考图的「优惠」本期不做）。
- 收支趋势折线图（当月每日）。
- 分类占比环形图。
- 底部 Tab：统计 / 标签。

### 4.4 资产页
- 净资产大数字（可切换显示/隐藏）。
- 总资产 / 总负债 双卡。
- 借出/借入（可映射到 `Assets:Receivable` / `Liabilities:Payable`，本期仅展示占位）。
- 账户分组（资产 / 负债 / …），每组可展开，账户行显示 icon + 余额。
- 底部「新增」按钮。

### 4.5 添加账户页（全屏）
- 图标 + 颜色选择（首行大 icon）。
- 账户类型（资产/负债/权益/收入/支出）。
- 名称、卡号（选填）、金额（期初余额）。
- 分组（选父账户/二级分类）、币种（默认 CNY）。
- 备注（选填）。
- 保存时生成期初余额交易。

### 4.6 记一笔页（全屏，简化模式）
- 顶部 Tab：支出 / 收入 / 转账。
- 大金额输入区。
- 分类选择（icon 网格 / 下拉）。
- 账户选择、日期、标签、描述。
- 高级入口：双录分录。
- 保存时按附录 B 生成 posting。

### 4.7 更多页
- 宫格或列表：设置、成员管理、导入导出、同步管理、报表分析、商品与价格、关于。

### 4.8 账本选择
- 底部弹窗（BottomSheet）：账本卡片列表，当前账本打勾。

---

## 5. PC 端各页面改造方案（概要，同 v1）

- **总览**：指标卡网格 + 收支趋势 + 最近交易。
- **交易流水**：筛选栏 + 列表/日历双 Tab + 类型推导。
- **记一笔**：弹窗简化模式 + 高级双录入口。
- **账户管理**：摘要 + 类型树 + 卡片。
- **报表分析**：分类分析 / 趋势分析 / 资产趋势。
- **分类/标签**：独立页面。

---

## 6. 前端文件改动清单

### 6.1 新增/修改文件（双端共用）

| 文件 | 动作 | 说明 |
|---|---|---|
| `core/models/account.dart` | 改 | 增 `icon`/`color`/`parentUuid`/`subType` |
| `core/models/transaction.dart` | 改 | 增 `tags: List<String>` |
| `widgets/account_icon.dart` | 新增 | 图标 + 颜色渲染组件 |
| `widgets/charts/*` | 新增 | 折线/柱状/环形图（推荐 `fl_chart`） |
| `screens/record/record_dialog.dart` | 新增 | PC 简化模式弹窗 |
| `screens/record/record_mobile_screen.dart` | 新增 | 移动端简化模式全屏页 |
| `screens/record/record_screen.dart` | 保留 | 高级双录入口 |
| `screens/categories/categories_screen.dart` | 新增 | 分类管理 |
| `screens/tags/tags_screen.dart` | 新增 | 标签管理 |

### 6.2 PC 端

| 文件 | 动作 | 说明 |
|---|---|---|
| `screens/dashboard/dashboard_screen.dart` | 改 | 卡片网格 + 趋势图 |
| `screens/transactions/transactions_screen.dart` | 改 | 筛选栏 + 双 Tab + 类型推导 |
| `screens/transactions/transactions_filter_bar.dart` | 新增 | 筛选组件 |
| `screens/accounts/accounts_screen.dart` | 改 | 摘要 + 类型树 + 卡片 |
| `screens/accounts/account_detail_screen.dart` | 新增 | 账户流水/对账单 |
| `screens/accounts/account_dialog.dart` | 新增 | PC 新建账户弹窗 |
| `screens/reports/reports_screen.dart` | 改 | 图表化 |
| `app/navigation.dart` | 改 | PC 新增分类/标签；移动端改 3 Tab |
| `screens/shell/desktop_shell.dart` | 改 | 侧边栏加入分类/标签 |

### 6.3 移动端

| 文件 | 动作 | 说明 |
|---|---|---|
| `screens/shell/mobile_shell.dart` | 改 | 底部 Tab 改为 账单/资产/更多 |
| `screens/mobile/bills_home_screen.dart` | 新增 | 日历首页 |
| `screens/mobile/bills_list_screen.dart` | 新增 | 列表视图 |
| `screens/mobile/stats_screen.dart` | 新增 | 移动端统计 |
| `screens/mobile/assets_screen.dart` | 新增 | 移动端资产页 |
| `screens/mobile/more_screen.dart` | 新增 | 更多入口页 |
| `screens/mobile/account_add_screen.dart` | 新增 | 移动端添加账户 |
| `screens/mobile/ledger_picker_sheet.dart` | 新增 | 账本选择 BottomSheet |

---

## 7. 后端 API / 数据库变更

> **不新增独立 REST 资源**。所有数据仍走本地 store + 同步协议。只扩展现有 schema 与同步实体字段。

### 7.1 `accounts` 表扩展
```sql
ALTER TABLE accounts ADD COLUMN icon TEXT;          -- 图标标识
ALTER TABLE accounts ADD COLUMN color TEXT;         -- 颜色 hex
ALTER TABLE accounts ADD COLUMN parent_uuid TEXT;   -- 父账户（三级结构用）
ALTER TABLE accounts ADD COLUMN sub_type TEXT;      -- 账户子类型
```
- 走迁移脚本 `internal/db/migrate.go`（003_account_visual）。
- 同步 push/pull 的 account 实体需携带上述字段。
- Beancount 导出忽略 icon/color/parent_uuid/sub_type。

### 7.2 `transactions` 表扩展
```sql
ALTER TABLE transactions ADD COLUMN tags TEXT;  -- JSON 数组，如 ["餐饮","出差"]
```
- 走迁移脚本 `004_txn_tags`。
- 同步 transaction 实体与在线 `POST /api/transactions` 请求体加 `tags`。
- Beancount 导出：交易行尾追加 `#tag1 #tag2`。

### 7.3 不做的接口/功能
- 暂不做服务端统计接口（保持离线可用），所有报表基于本地聚合。
- 期初余额由前端生成初始交易，不新增接口。
- 预算功能放到二期。
- 「计入总资产/记账时可选」开关放到二期。

---

## 8. 实施顺序

1. 账户 `icon/color/parent_uuid` + 分类页（为交易 UI 打底）
2. 新建账户 / 新建交易 弹窗与移动端页面（最高频操作）
3. 移动端底部导航改造（账单/资产/更多）+ 资产页
4. 移动端账单首页（日历）与列表视图
5. PC 总览页卡片化 + 趋势图
6. PC 交易流水筛选 + 日历视图
7. 统计分析页图表化（双端共用图表组件）
8. 标签页

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
