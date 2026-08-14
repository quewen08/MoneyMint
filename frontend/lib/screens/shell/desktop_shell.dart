/// PC 端主框架：左侧深色侧边栏（9 模块分组）+ 右侧内容区（顶栏 + 滚动内容）。
/// 侧边栏底部固定「记一笔」按钮；顶栏含页面标题、同步徽标、记一笔与账户入口。
import 'package:flutter/material.dart';
import '../../app/navigation.dart';
import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../widgets/sync_badge.dart';
import '../../widgets/pc.dart';

class DesktopShell extends StatelessWidget {
  final AppRoute route;
  final ValueChanged<AppRoute> onSelect;
  final VoidCallback onRecord;
  final Widget content;
  final Widget? floatingActionButton;
  const DesktopShell({
    super.key,
    required this.route,
    required this.onSelect,
    required this.onRecord,
    required this.content,
    this.floatingActionButton,
  });

  static const _sideBg = Color(0xFF15203B);
  static const _sideFg = Color(0xFFC7D0E0);

  @override
  Widget build(BuildContext context) {
    final ledger = AppScope.of(context).ledger;
    return Row(
      children: [
        _Sidebar(
          route: route,
          onSelect: onSelect,
          onRecord: onRecord,
        ),
        Expanded(
          child: Scaffold(
            appBar: AppBar(
              automaticallyImplyLeading: false,
              title: Text(routeTitles[route] ?? '家庭记账'),
              actions: [
                ListenableBuilder(
                  listenable: ledger,
                  builder: (ctx, _) => SyncBadgeFromController(ledger),
                ),
                const SizedBox(width: 12),
                PcButton('记一笔', icon: Icons.add, primary: true, onPressed: onRecord),
                const SizedBox(width: 12),
                IconButton(
                  tooltip: '设置',
                  icon: const Icon(Icons.person_outline),
                  onPressed: () => onSelect(AppRoute.settings),
                ),
                const SizedBox(width: 12),
              ],
            ),
            body: content,
            floatingActionButton: floatingActionButton,
          ),
        ),
      ],
    );
  }
}

class _Sidebar extends StatelessWidget {
  final AppRoute route;
  final ValueChanged<AppRoute> onSelect;
  final VoidCallback onRecord;
  const _Sidebar({
    required this.route,
    required this.onSelect,
    required this.onRecord,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 248,
      color: DesktopShell._sideBg,
      child: Column(
        children: [
          const SizedBox(height: 22),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 20),
            child: Row(
              children: [
                Container(
                  width: 30,
                  height: 30,
                  decoration: BoxDecoration(
                    color: AppColors.blue,
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: const Icon(Icons.account_balance_wallet,
                      color: Colors.white, size: 18),
                ),
                const SizedBox(width: 10),
                const Text('家庭记账',
                    style: TextStyle(
                        color: Colors.white,
                        fontSize: 16,
                        fontWeight: FontWeight.w800)),
              ],
            ),
          ),
          const SizedBox(height: 16),
          Expanded(
            child: ListView(
              padding: const EdgeInsets.symmetric(horizontal: 12),
              children: [
                for (final g in pcSidebar) ...[
                  Padding(
                    padding:
                        const EdgeInsets.fromLTRB(12, 14, 12, 6),
                    child: Text(g.title.toUpperCase(),
                        style: const TextStyle(
                            color: Color(0xFF7E8AA3),
                            fontSize: 11,
                            fontWeight: FontWeight.w700,
                            letterSpacing: 0.6)),
                  ),
                  for (final item in g.items)
                    _SideItem(
                      item: item,
                      selected: item.route == route,
                      onSelect: onSelect,
                    ),
                ],
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(16),
            child: SizedBox(
              width: double.infinity,
              child: ElevatedButton.icon(
                onPressed: onRecord,
                icon: const Icon(Icons.add, size: 18),
                label: const Text('记一笔'),
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.blue,
                  foregroundColor: Colors.white,
                  elevation: 0,
                  padding: const EdgeInsets.symmetric(vertical: 12),
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(10)),
                  textStyle: const TextStyle(
                      fontSize: 14, fontWeight: FontWeight.w700),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _SideItem extends StatelessWidget {
  final NavItem item;
  final bool selected;
  final ValueChanged<AppRoute> onSelect;
  const _SideItem({
    required this.item,
    required this.selected,
    required this.onSelect,
  });

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.only(bottom: 2),
        child: Material(
          color: selected ? AppColors.blue : Colors.transparent,
          borderRadius: BorderRadius.circular(8),
          child: InkWell(
            borderRadius: BorderRadius.circular(8),
            onTap: () => onSelect(item.route),
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
              child: Row(
                children: [
                  Icon(item.icon,
                      size: 19,
                      color: selected
                          ? Colors.white
                          : DesktopShell._sideFg),
                  const SizedBox(width: 12),
                  Text(item.label,
                      style: TextStyle(
                        fontSize: 13.5,
                        fontWeight:
                            selected ? FontWeight.w700 : FontWeight.w500,
                        color: selected
                            ? Colors.white
                            : DesktopShell._sideFg,
                      )),
                ],
              ),
            ),
          ),
        ),
      );
}
