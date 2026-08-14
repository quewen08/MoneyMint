# frontend — 家庭记账 Flutter Web 前端

类 Beancount 的 NAS 个人云记账应用的 Web 前端（PWA）。与 Go 后端通过 REST 通信，
支持离线记账 + 增量同步，对齐 `../docs/prototype/index.html` 设计稿。

## 架构（MVVM 分层 + 响应式双布局）

```
lib/
├── main.dart                 # 入口：AppScope 注入 + MaterialApp + AuthGate（登录门禁）
├── app/
│   ├── theme.dart            # 设计系统：AppColors / AppTheme.light / cardDecoration
│   ├── scope.dart            # AppScope(InheritedWidget)：根部提供 api/store/auth/ledger 单例
│   └── navigation.dart       # AppRoute 枚举 + isWide 断点(≥1024 PC / <1024 移动) + 导航分组
├── core/                     # 与 UI 无关的领域层（可单测）
│   ├── models/               # 本地数据模型：account / posting / transaction / pending_change / sync_models
│   ├── api/ledger_api.dart   # 认证 + 同步 + 导出 REST 客户端（token 存 localStorage）
│   ├── store/local_store.dart# sembast_web / IndexedDB 本地真相源 + Decimal 余额计算
│   └── sync/sync_service.dart# pull/push 编排 + 建账户/记账/冲正命令
├── viewmodels/               # ChangeNotifier 状态持有者
│   ├── auth_controller.dart  # 登录/注册/退出/初始化
│   └── ledger_controller.dart# 账户/交易/余额/净值/同步状态(SyncState) + 全部业务命令
├── widgets/                  # 可复用组件：common / sync_badge / balance_meter / pc(PC 端卡·图表·按钮)
├── utils/money.dart          # 金额格式化 / 币种符号 / isBalanced 借贷校验
└── screens/                  # 哑视图页面
    ├── auth / shell(响应路由+移动底栏+PC侧边栏) / dashboard / accounts / transactions / record / settings
    └── members / commodities / import_export / reports   # PC 专属模块（后 4 个为占位/可计算）
```

分层职责：Models 纯数据 → Core 网络/存储/同步 → ViewModels 状态与命令 → Views/Widgets 仅渲染。
视图通过 `ListenableBuilder`（`material.dart` 提供）订阅 controller。

**响应式**：`app_shell` 按 `isWide(context)`（断点 1024）在 `mobile_shell`（3 Tab 底栏 + FAB 记一笔）
与 `desktop_shell`（深色侧边栏 9 模块 + 顶栏）间切换；页面内部用 `isWide` 决定是否并排/限宽。
PC 专属模块落地策略：报表、导入导出为真实功能；成员管理、商品与价格为 `ComingSoon` 占位。

## 本地开发

```bash
export PATH="$PATH:/d/Environment/flutter/bin"   # 若 shell 未识别 flutter
flutter pub get
flutter analyze lib        # 编译级 0 error（dart:html / 弃用 API 的 info 提示属 Web 端预期）
flutter run -d chrome       # 本地调试；拉动浏览器宽度可切换 PC/移动布局
                          # 默认连 http://localhost:8080 后端，见 ledger_api.dart 的 kDefaultApiBase
```

构建 PWA 产物：`flutter build web --release`（产物在 `build/web/`，含自动生成的 Service Worker）。

## 约定

- 金额一律走 `decimal` 包，**禁止浮点参与金额运算**。
- 交易不可变：改账 = 新增冲正交易（`LocalTxn.reversedOf`），绝不覆盖。
- 在线记账 posting 用服务端 `account_id`；离线 push 交易 posting 用 `account_uuid`（见 `../AGENTS.md` §6.1）。
- 后端地址默认 `http://localhost:8080`，部署 NAS 时改 `core/api/ledger_api.dart` 的 `kDefaultApiBase` 或注入覆盖。
