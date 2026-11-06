/// 账户页（设计稿「账户」）：按 资产/负债/权益/收入/支出 分组展示，可跳转到该账户流水。
import 'package:decimal/decimal.dart';
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/navigation.dart';
import '../../app/theme.dart';
import '../../core/models/account.dart';
import '../../viewmodels/ledger_controller.dart';
import '../../utils/money.dart';
import '../../widgets/common.dart';
import '../../widgets/ledger_switcher.dart';
import '../../widgets/revamp.dart';
import '../transactions/transactions_screen.dart';
import '../categories/categories_screen.dart';
import 'account_detail_screen.dart';

class AccountsScreen extends StatefulWidget {
  const AccountsScreen({super.key});

  @override
  State<AccountsScreen> createState() => _AccountsScreenState();
}

class _AccountsScreenState extends State<AccountsScreen>
    with SingleTickerProviderStateMixin {
  late final TabController _tab;

  @override
  void initState() {
    super.initState();
    _tab = TabController(length: 2, vsync: this);
  }

  @override
  void dispose() {
    _tab.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final ledger = AppScope.of(context).ledger;
    final wide = isWide(context);
    return Column(
      children: [
        TabBar(
          controller: _tab,
          labelColor: AppColors.blue,
          unselectedLabelColor: AppColors.sub,
          tabs: const [Tab(text: '账户'), Tab(text: '分类')],
        ),
        Expanded(
          child: TabBarView(
            controller: _tab,
            children: [
              _AccountsList(ledger: ledger, wide: wide),
              const CategoriesScreen(),
            ],
          ),
        ),
      ],
    );
  }
}

class _AccountsList extends StatelessWidget {
  final LedgerController ledger;
  final bool wide;
  const _AccountsList({required this.ledger, required this.wide});

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: ledger,
      builder: (context, _) {
        if (ledger.loading) {
          return const Center(child: CircularProgressIndicator());
        }
        if (ledger.accounts.isEmpty) {
          return const Center(
            child: Padding(
              padding: EdgeInsets.all(24),
              child: Text('还没有账户，点「+」新建一个',
                  style: TextStyle(color: AppColors.sub)),
            ),
          );
        }
        final hasClosed = ledger.accounts.any((a) => a.isClosed);
        // F-5：移动端渐变头（全宽）+ 内容区；PC 保持白卡汇总。
        final groups = [
          if (hasClosed)
            Padding(
              padding: const EdgeInsets.only(bottom: 8),
              child: Row(
                children: [
                  const Spacer(),
                  Text('显示已关闭',
                      style: TextStyle(color: AppColors.sub, fontSize: 13)),
                  Switch(
                    value: ledger.showClosedAccounts,
                    onChanged: ledger.toggleShowClosedAccounts,
                  ),
                ],
              ),
            ),
          for (final type in LedgerController.typeOrder)
            if (ledger
                .accountsOfType(type, includeClosed: ledger.showClosedAccounts)
                .isNotEmpty)
              AccountGroup(
                title: _typeLabel(type),
                rows: ledger
                    .accountsOfType(type, includeClosed: ledger.showClosedAccounts)
                    .map((a) {
                  return LocalAccountRow(
                    name: a.display,
                    subtitle: a.isClosed
                        ? '已关闭${a.closeDate != null ? ' · ${a.closeDate}' : ''}'
                        : (a.restriction != null ? '限定 ${a.restriction}' : null),
                    balances: ledger.balances[a.uuid] ?? {},
                    dimmed: a.isClosed,
                    emoji: (a.icon?.isNotEmpty == true) ? a.icon! : null,
                    iconBg: AppColors.softOf(a.uuid, hexColor: a.color),
                    onTap: () => Navigator.push(
                      context,
                      MaterialPageRoute(
                        builder: (_) => AccountDetailScreen(account: a),
                      ),
                    ),
                    action: _AccountMenu(account: a),
                  );
                }).toList(),
              ),
        ];
        if (!wide) {
          return ListView(
            padding: const EdgeInsets.only(bottom: 24),
            children: [
              _HeroAssets(ledger: ledger),
              const SizedBox(height: 12),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16),
                child: Column(children: groups),
              ),
            ],
          );
        }
        return Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 900),
            child: ListView(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
              children: [
                _AssetSummary(ledger: ledger),
                const SizedBox(height: 8),
                ...groups,
              ],
            ),
          ),
        );
      },
    );
  }

  String _typeLabel(String type) => switch (type) {
        'Assets' => '资产',
        'Liabilities' => '负债',
        'Equity' => '权益',
        'Income' => '收入',
        'Expenses' => '支出',
        _ => type,
      };
}

/// 移动端资产页渐变头（F-5）：净资产大字 + 总资产/总负债。
class _HeroAssets extends StatelessWidget {
  final LedgerController ledger;
  const _HeroAssets({required this.ledger});

  String _money(Map<String, String> m) {
    if (m.isEmpty) return '—';
    return m.entries.map((e) => formatMoney(e.value, e.key)).join('  ');
  }

  /// 总负债按展示惯例取正数幅度；负债余额本身为负，这里显式 abs，不依赖格式化层吞符号。
  String _moneyAbs(Map<String, String> m) {
    if (m.isEmpty) return '—';
    return m.entries
        .map((e) => formatMoney(
            (Decimal.tryParse(e.value) ?? Decimal.zero).abs().toString(), e.key))
        .join('  ');
  }

  @override
  Widget build(BuildContext context) {
    final assets = ledger.byType['Assets'] ?? {};
    final liabs = ledger.byType['Liabilities'] ?? {};
    final net = ledger.netWorth;
    final auth = AppScope.of(context).auth;
    return HeroHeader(
      leading: const Text('资产',
          style: TextStyle(
              fontSize: 17, fontWeight: FontWeight.w700, color: Colors.white)),
      // 多账本全局化：四页头部统一账本 pill（F-5）。
      pill: ListenableBuilder(
        listenable: auth,
        builder: (ctx, _) => LedgerPill(
          ledgerName: auth.currentLedger?.name ?? '账本',
          onTap: () => showLedgerSwitcher(context),
        ),
      ),
      summary: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('净资产（资产 + 负债，负债为负）',
              style: TextStyle(
                  fontSize: 12, color: Colors.white.withValues(alpha: .72))),
          const SizedBox(height: 4),
          Text(
            net.isEmpty ? '¥ 0.00' : _money(net),
            style: const TextStyle(
              fontSize: 32,
              fontWeight: FontWeight.w800,
              letterSpacing: -0.5,
              color: Colors.white,
              fontFeatures: [FontFeature.tabularFigures()],
            ),
          ),
          const SizedBox(height: 14),
          Row(
            children: [
              Expanded(
                child: _heroCol('总资产', _money(assets)),
              ),
              Container(
                  width: 1,
                  height: 30,
                  color: Colors.white.withValues(alpha: .22),
                  margin: const EdgeInsets.symmetric(horizontal: 14)),
              Expanded(
                child: _heroCol('总负债', _moneyAbs(liabs)),
              ),
            ],
          ),
          const SizedBox(height: 30), // 给上浮卡让位
        ],
      ),
    );
  }

  Widget _heroCol(String lb, String v) => Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(lb,
              style: TextStyle(
                  fontSize: 12, color: Colors.white.withValues(alpha: .72))),
          const SizedBox(height: 3),
          Text(
            v,
            style: const TextStyle(
              fontSize: 16,
              fontWeight: FontWeight.w700,
              color: Colors.white,
              fontFeatures: [FontFeature.tabularFigures()],
            ),
            overflow: TextOverflow.ellipsis,
          ),
        ],
      );
}

/// 顶部汇总卡（PC 宽屏）：总资产 / 总负债 / 净资产。
class _AssetSummary extends StatelessWidget {
  final LedgerController ledger;
  const _AssetSummary({required this.ledger});

  String _money(Map<String, String> m) {
    if (m.isEmpty) return '—';
    return m.entries.map((e) => formatMoney(e.value, e.key)).join('  ');
  }

  /// 总负债按展示惯例取正数幅度；负债余额本身为负，这里显式 abs，不依赖格式化层吞符号。
  String _moneyAbs(Map<String, String> m) {
    if (m.isEmpty) return '—';
    return m.entries
        .map((e) => formatMoney(
            (Decimal.tryParse(e.value) ?? Decimal.zero).abs().toString(), e.key))
        .join('  ');
  }

  @override
  Widget build(BuildContext context) {
    final assets = ledger.byType['Assets'] ?? {};
    final liabs = ledger.byType['Liabilities'] ?? {};
    final net = ledger.netWorth;
    return AppCard(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text('净资产（资产 + 负债，负债为负）', style: AppTheme.muted),
          const SizedBox(height: 4),
          Text(
            net.isEmpty ? '¥ 0.00' : _money(net),
            style: const TextStyle(
              fontSize: 30,
              fontWeight: FontWeight.w800,
              letterSpacing: -0.5,
            ),
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              Expanded(
                child: _SummaryItem(label: '总资产', value: _money(assets), color: AppColors.blue),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: _SummaryItem(label: '总负债', value: _moneyAbs(liabs), color: AppColors.warn),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _SummaryItem extends StatelessWidget {
  final String label;
  final String value;
  final Color color;
  const _SummaryItem({required this.label, required this.value, required this.color});

  @override
  Widget build(BuildContext context) => Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(width: 4, height: 12, decoration: BoxDecoration(color: color, borderRadius: BorderRadius.circular(2))),
              const SizedBox(width: 6),
              Text(label, style: AppTheme.muted),
            ],
          ),
          const SizedBox(height: 4),
          Text(
            value,
            style: const TextStyle(
              fontSize: 16,
              fontWeight: FontWeight.w700,
              fontFeatures: [FontFeature.tabularFigures()],
            ),
            overflow: TextOverflow.ellipsis,
          ),
        ],
      );
}

/// 账户行尾操作菜单：查看流水 / 关闭账户（0.4-A：close 语义，保留历史）。
class _AccountMenu extends StatelessWidget {
  final LocalAccount account;
  const _AccountMenu({required this.account});

  @override
  Widget build(BuildContext context) {
    final ledger = AppScope.of(context).ledger;
    return PopupMenuButton<String>(
      tooltip: '账户操作',
      icon: const Icon(Icons.more_vert, size: 18, color: AppColors.sub),
      padding: EdgeInsets.zero,
      constraints: const BoxConstraints(minWidth: 120),
      onSelected: (v) {
        if (v == 'flow') {
          Navigator.push(
            context,
            MaterialPageRoute(
              builder: (_) => TransactionsScreen(filterAccountUuid: account.uuid),
            ),
          );
        } else if (v == 'detail') {
          Navigator.push(
            context,
            MaterialPageRoute(
              builder: (_) => AccountDetailScreen(account: account),
            ),
          );
        } else if (v == 'default') {
          _setDefault(context);
        } else if (v == 'close') {
          runCloseAccountFlow(context, ledger, account);
        }
      },
      itemBuilder: (_) {
        final online = ledger.online;
        final items = <PopupMenuItem<String>>[
          const PopupMenuItem(value: 'detail', child: Text('账户详情')),
          const PopupMenuItem(value: 'flow', child: Text('查看流水')),
        ];
        if (account.type == 'Expenses' || account.type == 'Income') {
          items.add(PopupMenuItem(
            value: 'default',
            child: Text(account.type == 'Expenses' ? '设为默认支出账户' : '设为默认收入账户'),
          ));
        }
        items.add(PopupMenuItem(
          value: 'close',
          enabled: online,
          child: Text(
            online ? '关闭账户' : '关闭账户（需联网）',
            style: TextStyle(color: online ? AppColors.warn : AppColors.sub),
          ),
        ));
        return items;
      },
    );
  }

  Future<void> _setDefault(BuildContext context) async {
    final kind = account.type == 'Expenses' ? 'expense' : 'income';
    try {
      await AppScope.of(context).ledger.setDefaultAccount(kind, account.uuid);
      if (!context.mounted) return;
      showAppSnack(context, '已设为默认${account.type == 'Expenses' ? '支出' : '收入'}账户');
    } catch (e) {
      if (!context.mounted) return;
      showAppSnack(context, '设置失败: $e', warn: true);
    }
  }
}
