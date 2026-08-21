/// 应用主框架（响应式路由）：>1024 走 PC 侧边栏布局，否则走移动底栏布局。
/// 监听浏览器网络事件，联网自动同步；启动即初始化账本数据。
import 'dart:html' as html;
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/navigation.dart';
import '../../app/theme.dart';
import '../../viewmodels/ledger_controller.dart';
import '../record/record_screen.dart';
import '../record/record_dialog.dart';
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
  bool _initialized = false;
  LedgerController? _ledger;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (!_initialized) {
      _initialized = true;
      _ledger = AppScope.of(context).ledger;
      _ledger!.init();
      html.window.onOnline.listen((_) {
        if (!mounted) return;
        _ledger?.syncNow();
      });
      html.window.onOffline.listen((_) {
        if (!mounted) return;
        _ledger?.markOffline();
      });
    }
  }

  void _select(AppRoute r) => setState(() => _route = r);

  void _goRecord() {
    if (isWide(context)) {
      showDialog(context: context, builder: (_) => const RecordDialog());
    } else {
      Navigator.push<bool>(
        context,
        MaterialPageRoute(builder: (_) => const RecordScreen()),
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
