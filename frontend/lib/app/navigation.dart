/// 响应式导航模型：定义目的地、移动端 3 个底栏 Tab、PC 端侧边栏 9 模块。
/// 断点策略（见设计稿「响应式」注解）：>1024 走 PC 布局；<1024 降级为移动布局
/// （含 768–1024 中间宽度）。记一笔是动作而非导航项（移动 FAB / PC 顶栏按钮）。
import 'package:flutter/material.dart';

/// 所有可导航目的地。
enum AppRoute {
  dashboard,
  transactions,
  accounts,
  members,
  commodities,
  importExport,
  reports,
  sync,
  settings,
}

/// PC / 宽屏断点（px）。
const kDesktopBreakpoint = 1024;

/// 是否按 PC 宽屏布局渲染。
bool isWide(BuildContext context) =>
    MediaQuery.of(context).size.width >= kDesktopBreakpoint;

/// 单个导航项。
class NavItem {
  final AppRoute route;
  final String label;
  final IconData icon;
  const NavItem(this.route, this.label, this.icon);
}

/// 移动端底部 3 个 Tab（设计稿：首页 / 流水 / 我的；去掉「账户」）。
const mobileTabs = [
  NavItem(AppRoute.dashboard, '首页', Icons.home_outlined),
  NavItem(AppRoute.transactions, '流水', Icons.receipt_long_outlined),
  NavItem(AppRoute.settings, '我的', Icons.person_outline),
];

/// PC 端侧边栏分组（9 模块：概览 / 管理 / 工具 / 系统）。
class NavGroup {
  final String title;
  final List<NavItem> items;
  const NavGroup(this.title, this.items);
}

const pcSidebar = [
  NavGroup('概览', [
    NavItem(AppRoute.dashboard, '仪表盘', Icons.home_outlined),
    NavItem(AppRoute.transactions, '交易流水', Icons.receipt_long_outlined),
  ]),
  NavGroup('管理', [
    NavItem(AppRoute.accounts, '账户管理', Icons.account_balance_wallet_outlined),
    NavItem(AppRoute.members, '成员管理', Icons.group_outlined),
    NavItem(AppRoute.commodities, '商品与价格', Icons.paid_outlined),
  ]),
  NavGroup('工具', [
    NavItem(AppRoute.importExport, '导入与导出', Icons.swap_horiz_outlined),
    NavItem(AppRoute.reports, '报表分析', Icons.bar_chart_outlined),
    NavItem(AppRoute.sync, '同步管理', Icons.sync_outlined),
  ]),
  NavGroup('系统', [
    NavItem(AppRoute.settings, '设置', Icons.settings_outlined),
  ]),
];

/// 各目的地标题（PC 顶栏 / AppBar 展示）。
const routeTitles = {
  AppRoute.dashboard: '仪表盘',
  AppRoute.transactions: '交易流水',
  AppRoute.accounts: '账户管理',
  AppRoute.members: '成员管理',
  AppRoute.commodities: '商品与价格',
  AppRoute.importExport: '导入与导出',
  AppRoute.reports: '报表分析',
  AppRoute.sync: '同步管理',
  AppRoute.settings: '设置',
};
