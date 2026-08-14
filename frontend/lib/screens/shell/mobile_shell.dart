/// 移动端主框架：底部 3 个 Tab（首页 / 流水 / 我的）+ 记一笔 FAB。
/// 顶部 AppBar 显示当前页标题与同步徽标。
import 'package:flutter/material.dart';
import '../../app/navigation.dart';
import '../../app/scope.dart';
import '../../widgets/sync_badge.dart';

class MobileShell extends StatelessWidget {
  final AppRoute route;
  final ValueChanged<AppRoute> onSelect;
  final VoidCallback onRecord;
  final Widget content;
  const MobileShell({
    super.key,
    required this.route,
    required this.onSelect,
    required this.onRecord,
    required this.content,
  });

  int _tabIndex() {
    final i = mobileTabs.indexWhere((t) => t.route == route);
    return i < 0 ? 0 : i;
  }

  @override
  Widget build(BuildContext context) {
    final ledger = AppScope.of(context).ledger;
    return Scaffold(
      appBar: AppBar(
        title: Text(routeTitles[route] ?? '家庭记账'),
        actions: [
          Padding(
            padding: const EdgeInsets.only(right: 8),
            child: ListenableBuilder(
              listenable: ledger,
              builder: (ctx, _) => SyncBadgeFromController(ledger),
            ),
          ),
        ],
      ),
      body: content,
      bottomNavigationBar: NavigationBar(
        selectedIndex: _tabIndex(),
        onDestinationSelected: (i) => onSelect(mobileTabs[i].route),
        destinations: [
          for (final t in mobileTabs)
            NavigationDestination(
              icon: Icon(t.icon),
              label: t.label,
            ),
        ],
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: onRecord,
        icon: const Icon(Icons.add),
        label: const Text('记一笔'),
      ),
    );
  }
}
