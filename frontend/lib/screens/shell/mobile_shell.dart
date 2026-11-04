/// 移动端主框架（v2 改版）：底部 4 Tab + 中央「记一笔」凸起 FAB。
/// Tab：账单 / 资产 / （＋）/ 报表 / 更多。流水从账单页右上 📋 或资产账户详情进入。
/// 顶部 AppBar 保留：页标题、同步徽标、过渡期流水入口（F-6 落地账单页内入口后移除）。
import 'package:flutter/material.dart';
import '../../app/navigation.dart';
import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../widgets/ledger_switcher.dart';
import '../../widgets/revamp.dart';
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

  @override
  Widget build(BuildContext context) {
    final ledger = AppScope.of(context).ledger;
    final auth = AppScope.of(context).auth;
    final tab = mobileTabs.where((t) => t.route == route).toList();
    // 账单 / 资产页自带渐变头（HeroHeader 内已放白字 pill），此处不重复展示；
    // 报表 / 更多没有渐变头，pill 落在 AppBar 右侧。
    final showPill = route == AppRoute.reports || route == AppRoute.settings;
    return Scaffold(
      appBar: AppBar(
        title: Text(tab.isNotEmpty
            ? tab.first.label
            : (routeTitles[route] ?? '家庭记账')),
        actions: [
          if (showPill)
            ListenableBuilder(
              listenable: auth,
              builder: (ctx, _) => Padding(
                padding: const EdgeInsets.only(right: 8),
                child: Center(
                  child: LedgerPill(
                    light: true,
                    ledgerName: auth.currentLedger?.name ?? '账本',
                    onTap: () => showLedgerSwitcher(context),
                  ),
                ),
              ),
            ),
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
      bottomNavigationBar: _RevampTabBar(
        route: route,
        onSelect: onSelect,
        onRecord: onRecord,
      ),
    );
  }
}

/// v2 底栏：账单 / 资产 / （＋凸起）/ 报表 / 更多。
class _RevampTabBar extends StatelessWidget {
  final AppRoute route;
  final ValueChanged<AppRoute> onSelect;
  final VoidCallback onRecord;
  const _RevampTabBar({
    required this.route,
    required this.onSelect,
    required this.onRecord,
  });

  @override
  Widget build(BuildContext context) {
    Widget tb(int i) {
      final t = mobileTabs[i];
      final on = mobileTabs[i].route == route;
      return Expanded(
        child: InkWell(
          onTap: () => onSelect(t.route),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.start,
            children: [
              const SizedBox(height: 9),
              Icon(t.icon, size: 22,
                  color: on ? AppColors.blue : AppColors.sub),
              const SizedBox(height: 2),
              Text(t.label,
                  style: TextStyle(
                      fontSize: 10.5,
                      color: on ? AppColors.blue : AppColors.sub,
                      fontWeight: on ? FontWeight.w700 : FontWeight.w400)),
            ],
          ),
        ),
      );
    }

    return Container(
      decoration: const BoxDecoration(
        color: AppColors.card,
        border: Border(top: BorderSide(color: Color(0xFFECEDF2), width: .5)),
      ),
      child: SafeArea(
        top: false,
        child: SizedBox(
          height: 64,
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              tb(0),
              tb(1),
              Expanded(
                child: Transform.translate(
                  offset: const Offset(0, -18),
                  child: GestureDetector(
                    onTap: onRecord,
                    child: Center(
                      child: Container(
                        width: 54,
                        height: 54,
                        decoration: BoxDecoration(
                          gradient: AppColors.fabGradient,
                          shape: BoxShape.circle,
                          border: Border.all(color: AppColors.bg, width: 4),
                          boxShadow: const [
                            BoxShadow(
                                color: Color(0x6B3B6EF6),
                                blurRadius: 18,
                                offset: Offset(0, 6)),
                          ],
                        ),
                        child: const Icon(Icons.add, color: Colors.white, size: 28),
                      ),
                    ),
                  ),
                ),
              ),
              tb(2),
              tb(3),
            ],
          ),
        ),
      ),
    );
  }
}
