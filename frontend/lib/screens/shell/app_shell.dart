/// 应用主框架（响应式路由）：>1024 走 PC 侧边栏布局，否则走移动底栏布局。
/// 监听浏览器网络事件，联网自动同步；启动即初始化账本数据。
import 'dart:html' as html;
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/navigation.dart';
import '../../app/theme.dart';
import '../../viewmodels/ledger_controller.dart';
import '../record/record_dialog.dart';
import '../record/record_sheet.dart';
import '../dashboard/dashboard_screen.dart';
import '../transactions/transactions_screen.dart';
import '../accounts/accounts_screen.dart';
import '../accounts/account_dialog.dart';
import '../members/members_screen.dart';
import '../commodities/commodities_screen.dart';
import '../import_export/import_export_screen.dart';
import '../reports/reports_screen.dart';
import '../settings/sync_screen.dart';
import '../settings/settings_screen.dart';
import '../accounts/account_edit_screen.dart';
import 'mobile_shell.dart';
import 'desktop_shell.dart';

class AppShell extends StatefulWidget {
  const AppShell({super.key});

  @override
  State<AppShell> createState() => _AppShellState();
}

class _AppShellState extends State<AppShell> {
  AppRoute _route = AppRoute.dashboard;
  /// 已执行过 init 的控制器实例（原因同 AuthGate：热重载会重建 AppScope，
  /// 一次性 bool 门闩会让新控制器永远不被 init）。
  LedgerController? _initedLedger;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final ledger = AppScope.of(context).ledger;
    if (_initedLedger == ledger) return;
    _initedLedger = ledger;
    if (kDebugMode) {
      debugPrint('[shell] 挂载，init ledger=#${identityHashCode(ledger)}');
    }
    ledger.init();
    html.window.onOnline.listen((_) {
      if (!mounted) return;
      ledger.syncNow();
    });
    html.window.onOffline.listen((_) {
      if (!mounted) return;
      ledger.markOffline();
    });
  }

  void _select(AppRoute r) => setState(() => _route = r);

  void _goRecord() {
    if (isWide(context)) {
      showDialog(context: context, builder: (_) => const RecordDialog());
    } else {
      showModalBottomSheet(
        context: context,
        isScrollControlled: true,
        backgroundColor: Colors.transparent,
        builder: (_) => const RecordSheet(),
      );
    }
  }

  Widget _content() {
    switch (_route) {
      case AppRoute.dashboard:
        return const DashboardScreen();
      case AppRoute.transactions:
        return const TransactionsScreen();
      case AppRoute.accounts:
        return const AccountsScreen();
      case AppRoute.members:
        return const MembersScreen();
      case AppRoute.commodities:
        return const CommoditiesScreen();
      case AppRoute.importExport:
        return const ImportExportScreen();
      case AppRoute.reports:
        return const ReportsScreen();
      case AppRoute.sync:
        return const SyncScreen(embedded: true);
      case AppRoute.settings:
        return const SettingsScreen();
    }
  }

  void _goAccountEdit(BuildContext context) {
    if (isWide(context)) {
      showDialog(context: context, builder: (_) => const AccountDialog());
    } else {
      Navigator.push<bool>(
        context,
        MaterialPageRoute(builder: (_) => const AccountEditScreen()),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final content = _content();
    final wide = isWide(context);
    final fab = wide && _route == AppRoute.accounts
        ? FloatingActionButton(
            onPressed: () => _goAccountEdit(context),
            backgroundColor: AppColors.blue,
            child: const Icon(Icons.add, color: Colors.white),
          )
        : null;
    if (wide) {
      return DesktopShell(
        route: _route,
        onSelect: _select,
        onRecord: _goRecord,
        content: content,
        floatingActionButton: fab,
      );
    }
    return MobileShell(
      route: _route,
      onSelect: _select,
      onRecord: _goRecord,
      content: content,
    );
  }
}
