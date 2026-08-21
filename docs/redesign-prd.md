# FamBean 全端体验优化 PRD（v2）

> 版本：v2.0 ｜ 状态：评审通过，进入研发
> 关联方案：`docs/redesign-plan.md`、设计稿 `docs/prototype/pc-redesign.html` / `docs/prototype/mobile-redesign.html`
> 核心约束：**UI 可借鉴成熟产品，Beancount 复式账芯与「导出 `.beancount` 必须过 `bean-check`」铁律不变。**

---

## 1. 背景与目标

家庭 NAS 云记账应用 FamBean 已完成账芯（复式记账、离线同步、多账本、成员权限、Beancount 导出）。
当前前端为功能验证版，体验粗糙：无分类/标签体系、记一笔需手写分录、无图表、移动端导航不聚焦。

**目标**：对齐 ezBookkeeping 等成熟产品的信息架构与交互，补齐「分类 / 标签 / 图标 / 图表 / 简化记账」等家庭记账高频能力，
让用户无需理解复式记账也能流畅记账，同时保持账芯 100% Beancount 兼容。

**非目标（本期不做，见 §9）**：预算模块、服务端统计接口、账户「计入总资产/记账时可选」开关、跨币种成本注解、导入（OFX/CSV）。

---

## 2. 用户故事

- 作为家庭成员，我希望在手机上点几下就能记一笔支出，而不用手写借贷分录。
- 作为记账用户，我希望交易能打标签、按分类（餐饮/交通…）归类，方便日后筛选与统计。
- 作为账本 owner，我希望账户和分类有图标和颜色，一眼区分。
- 作为长期用户，我希望在 PC 上看到资产趋势、收支分类占比等图表。
- 作为移动端用户，我希望底部「账单 / 资产 / 更多」导航聚焦、日历首页直觉。

---

## 3. 功能清单（EARS）

### 3.1 分类体系（复用账户模型）
- **Ubiquitous**：系统始终将「分类」实现为 `Expenses` / `Income` 类型账户，不引入独立分类实体。
- **State-driven**：当账户 `type = Expenses` 或 `Income` 且 `parent_uuid` 为空时，系统 shall 将其视为「二级分类」。
- **State-driven**：当账户 `parent_uuid` 指向某个二级分类账户时，系统 shall 将其视为「三级分类」（系统内置 + 用户可添加）。
- **Ubiquitous**：系统 shall 在新建账本时 seed 一组默认二级分类（餐饮/食品/交通/购物…与薪资/奖金…）。

### 3.2 账户视觉字段
- **Ubiquitous**：系统 shall 为账户存储 `icon`（图标标识）、`color`（hex 颜色）、`parent_uuid`（父账户）、`sub_type`（子类型）。
- **Ubiquitous**：当账户未设置 `icon` 时，UI shall 回退到按账户类型/名称派生的默认图标。
- **Ubiquitous**：Beancount 导出 shall 忽略 `icon`/`color`/`parent_uuid`/`sub_type`，仅输出标准账户声明。

### 3.3 简化记一笔（核心）
- **Event-driven**：当用户在「支出/收入/转账」弹窗（PC）或全屏页（移动端）点击保存时，系统 shall 按附录 B 生成标准复式 posting 并提交。
- **Ubiquitous**：系统 shall 保留「高级」双录分录入口，供熟悉 Beancount 的用户使用。
- **Unwanted**：若生成的 posting 按币种借贷和不为 0，则系统 shall 拒绝提交并返回平衡校验错误（复用现有 `createTransaction` 校验）。

### 3.4 标签
- **Event-driven**：当用户在记一笔时填写标签，系统 shall 将标签列表写入该交易。
- **Ubiquitous**：系统 shall 支持按标签筛选交易列表与统计。
- **Ubiquitous**：Beancount 导出 shall 在交易头行之下追加 `tags: "tag1 tag2"` 元数据行。
  > 设计权衡：Beancount 的 `#tag` 头缀语法仅允许 ASCII 字符（`[A-Za-z0-9\-_/.]+`），
  > 不支持中文标签；而本应用需支持中文标签，故统一以元数据 `tags: "..."` 形式写出——
  > 既能保留中文又能通过 `bean-check`。代价是该标签不会被 Beancount 识别为 `#tag` 查询键，
  > 但作为记账留档与兼容导出完全满足需求。

### 3.5 期初余额
- **Event-driven**：当用户在新建账户时填写「期初余额」，系统 shall 额外生成一笔 `Equity:Opening-Balances → 该账户` 的初始交易，使借贷平衡。

### 3.6 图表与总览（本地聚合）
- **Ubiquitous**：总览页 shall 以卡片网格展示净资产/总资产/总负债/本月收支，并展示近 12 个月收支趋势图。
- **Ubiquitous**：报表页 shall 提供分类占比（环形）、收支趋势（柱状/折线）、资产趋势三类图表。
- **Ubiquitous**：所有图表 shall 基于本地 `local_store` 数据聚合计算，不依赖服务端统计接口（保持离线可用）。

### 3.7 移动端导航
- **Ubiquitous**：移动端 shall 使用底部三 Tab：账单 / 资产 / 更多。
- **Event-driven**：当用户打开「账单」Tab，系统 shall 默认展示日历视图，并允许切换到列表视图与统计页。

---

## 4. 流程说明（简化记一笔）

```
用户选择 支出/收入/转账
  → 输入金额、选分类(Expenses/Income账户)/账户(Assets/Liabilities)、选日期、填标签/描述
  → [保存]
     支出 X：Assets/负债账户 -X ＋ Expenses:分类 +X
     收入 X：Income:分类 -X ＋ Assets/负债账户 +X
     转账 X：来源账户 -X ＋ 目标账户 +X
  → 复用 createTransaction 平衡校验（按币种借贷和为 0）
  → 落本地 store + 入待推送队列 + 自动同步
```

---

## 5. 数据模型变更（后端）

> 不新增独立 REST 资源，全部走现有同步协议（push/pull）。仅扩展 schema 与同步实体字段。
> 迁移脚本：`internal/db/migrate.go`，幂等、记录 `schema_migrations`。

**迁移 004 — 账户视觉字段**
```sql
ALTER TABLE accounts ADD COLUMN icon TEXT;
ALTER TABLE accounts ADD COLUMN color TEXT;
ALTER TABLE accounts ADD COLUMN parent_uuid TEXT;
ALTER TABLE accounts ADD COLUMN sub_type TEXT;
```

**迁移 005 — 交易标签**
```sql
ALTER TABLE transactions ADD COLUMN tags TEXT;  -- JSON 数组字符串，如 ["餐饮","出差"]
```

**同步实体追加字段**
- account 实体：`icon` / `color` / `parent_uuid` / `sub_type`
- transaction 实体：`tags`（JSON 数组）

---

## 6. 交互与边界

- 分类删除：本质为删除 `Expenses`/`Income` 账户，复用账户软删（连带其引用交易）。
- 标签为空数组时导出不输出 `tags` 元数据行。
- 图标/颜色为纯展示字段，缺失不影响任何记账与导出逻辑。
- 旧库升级：迁移对既有账户/交易新列默认 NULL/空，向前兼容。

---

## 7. 验收标准

### 7.1 后端
- [ ] 全新库与既有库（含 003 之前数据）均能成功应用迁移 004/005，`schema_migrations` 记录存在。
- [ ] `accounts` 的 `icon/color/parent_uuid/sub_type` 经 push/create 落库，经 pull 回到其他端。
- [ ] `transactions` 的 `tags` 经 push/create 落库，经 pull 回到其他端。
- [ ] 导出 `.beancount` 在有标签时含 `tags: "..."` 元数据行（中文标签亦可通过），且 `bean-check` 退出码为 0。
- [ ] 现有借贷平衡校验对带 tags 的交易仍然生效。

### 7.2 前端
- [ ] 记一笔弹窗（PC）/ 全屏页（移动端）能生成合法复式 posting 并平衡通过。
- [ ] 分类页可展示二级/三级分类，新增三级分类落库并同步。
- [ ] 账户/分类显示 icon 与色块（无 icon 时回退默认）。
- [ ] 交易列表/统计可按标签筛选。
- [ ] 移动端底部三 Tab 生效，账单默认日历视图。
- [ ] `flutter analyze` 无错误。

---

## 8. 实施顺序（研发拆解）

1. 后端迁移 004/005 + domain/repository/service/handler/export 适配（**本期先做，所有 UI 的地基**）。
2. 前端模型（LocalAccount/LocalTxn）+ SyncService 适配。
3. 账户图标组件 + 分类页 + 新建账户/新建交易弹窗。
4. 移动端底部导航 + 资产页 + 账单首页（日历/列表）。
5. PC 总览卡片化 + 趋势图。
6. PC 交易流水筛选 + 日历视图。
7. 报表图表化。
8. 标签页。

---

## 附录 A：交易类型推导

| posting 账户类型组合 | 显示类型 |
|---|---|
| 含 Assets/Liabilities 减少 + 含 Expenses 增加 | 支出 |
| 含 Income 减少 + 含 Assets/Liabilities 增加 | 收入 |
| 两端均为 Assets 或均为 Liabilities | 转账 |
| 其他 | 其他 |

## 附录 B：简化模式 posting 生成

- 支出 X：`Assets/负债账户 -X` + `Expenses:分类 +X`
- 收入 X：`Income:分类 -X` + `Assets/负债账户 +X`
- 转账 X：`来源账户 -X` + `目标账户 +X`

> 所有金额按 commodity 分组的借贷和必须为 0，复用现有 `createTransaction` 平衡校验。
