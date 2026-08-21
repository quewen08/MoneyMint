# FamBean 全端优化 v2 — 验收清单（0.3.0）

> 关联：PRD `docs/redesign-prd.md`、变更 `CHANGELOG.md`（[0.3.0]）
> 状态：**0.3.0 里程碑完成**（账芯 + 数据层 + 核心交互弹窗）。
> 剩余 UI 屏（移动导航 / PC 总览图表 / 交易筛选+日历 / 标签页）见文末 Backlog，归入下个迭代。

---

## A. 后端（已自动验证：`go build ./...` + `go test ./internal/...` 全绿）

| # | 验收项 | 验证方式 | 状态 |
|---|---|---|---|
| A1 | 迁移 004/005 幂等可重入，旧库升级无碍 | `go test ./internal/...`；手动对既有库重启服务 | ✅ |
| A2 | 账户 `icon/color/parent_uuid/sub_type` 经 push/create 落库、经 pull 回传 | 同步单测 + 端到端 smoke | ✅ |
| A3 | 交易 `tags` 经 push/create 落库、经 pull 回传 | 同步单测 + 端到端 smoke | ✅ |
| A4 | 导出 `.beancount` 在有标签时含 `tags: "..."` 元数据行，**中文标签亦可通过** | `TestExportTags` 跑 `bean-check` 退出码 0 | ✅ |
| A5 | 借贷平衡校验对带 tags 交易仍生效 | 现有 `createTransaction` 校验 + 单测 | ✅ |

### 自测命令（后端）
```bash
export PATH="$PATH:/d/Environment/go/bin"
cd D:/Project/Self/FamBean
go build ./... && go test ./internal/... -run 'TestExport|TestCreate' -v
```

---

## B. 前端（已你本地 `flutter analyze lib` 验证：No issues found）

| # | 验收项 | 自测路径 | 状态 |
|---|---|---|---|
| B1 | 简化记一笔弹窗（PC）/ 全屏页（移动）生成合法复式 posting 并平衡通过 | App → 记一笔 → 支出/收入/转账 | ⏳ 需手测 |
| B2 | 新建账户支持 emoji 图标 / 颜色 / 父分类 / 期初余额（自动生成 Opening-Balances 冲抵） | 资产 → 账户 → 新建 | ⏳ 需手测 |
| B3 | 分类页展示二级/三级分类，新增三级分类落库并同步 | 资产 → 分类 | ⏳ 需手测 |
| B4 | 账户/分类显示图标 + 色块（无 icon 回退默认） | 资产列表 | ⏳ 需手测 |
| B5 | 离线记账 → 联网自动 pull/push 携带新字段（icon/color/tags） | 断网记一笔 → 联网同步 | ⏳ 需手测 |
| B6 | `flutter analyze lib` 无错误 | 见上 | ✅ |

### 自测命令（前端）
```bash
export PATH="$PATH:/d/Environment/go/bin:/d/Environment/flutter/bin"
cd D:/Project/Self/FamBean/frontend
flutter analyze lib
flutter test
```

---

## C. 导出兼容性（硬约束复查）
- [x] 中文标签导出用 `tags: "..."` 元数据，非 `#tag`（ASCII-only 限制）。
- [x] 导出文本经 `bean-check` 退出码 0（含带标签样本）。
- [ ] 真机/浏览器端到端导出并过 `bean-check`（建议上线前手测一次）。

---

## D. 下个迭代 Backlog（待排期）

> 以下均为纯前端 UI 模块；本沙箱 Dart/Flutter 不可用，需在你本地 `flutter analyze` 验证。

1. **移动端导航重构**：底部三 Tab（账单 / 资产 / 更多），账单默认日历视图、可切列表/统计。
2. **PC 总览卡片化**：净资产 / 总资产 / 总负债 / 本月收支卡片 + 近 12 月收支趋势图（本地 `local_store` 聚合，离线可用）。
3. **PC 交易流水**：按账户/分类/标签/日期区间筛选 + 日历视图。
4. **标签页**：按标签筛选交易列表与统计。
5. **报表图表化**：分类占比（环形）/ 收支趋势（柱/折线）/ 资产趋势。

建议实现顺序沿用 PRD §8：移动导航 → PC 总览 → PC 交易筛选 → 报表 → 标签页。
