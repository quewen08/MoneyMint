/// 首页（设计稿「首页」）：净资产大字号 + 资产/负债/权益分组 + 最近交易。
/// 0.4-B：顶部加四宫格指标 + 近 12 月趋势迷你折线。
import 'package:decimal/decimal.dart';
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/navigation.dart';
import '../../app/theme.dart';
import '../../core/models/transaction.dart';
import '../../viewmodels/ledger_controller.dart';
import '../../utils/money.dart';
import '../../widgets/calendar_view.dart';
import '../../widgets/charts.dart';
import '../../widgets/common.dart';
import '../../widgets/privacy.dart';
import '../transactions/txn_detail_screen.dart';
import '../transactions/transactions_screen.dart';

class DashboardScreen extends StatelessWidget {
  const DashboardScreen({super.key});

  @override
  Widget build(BuildContext context) {
    // 0.4-D：<1024 走移动端「账单首页」，>1024 保持原 PC 仪表盘。
    if (!isWide(context)) {
      return const _MobileBillHome();
    }
    final ledger = AppScope.of(context).ledger;
    final wide = isWide(context); // 走到这里必为宽屏（窄屏已提前返回）
    return ListenableBuilder(
      listenable: ledger,
      builder: (context, _) {
        if (ledger.loading) {
          return const Center(child: CircularProgressIndicator());
        }
        final assetRows =
            ledger.accountsOfType('Assets').map(_toRow(ledger)).toList();
        final liabRows =
            ledger.accountsOfType('Liabilities').map(_toRow(ledger)).toList();
        final equityRows =
            ledger.accountsOfType('Equity').map(_toRow(ledger)).toList();
        final groups = <Widget>[
          if (wide)
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(child: AccountGroup(title: '资产', rows: assetRows)),
                const SizedBox(width: 12),
                Expanded(child: AccountGroup(title: '负债', rows: liabRows)),
                const SizedBox(width: 12),
                Expanded(child: AccountGroup(title: '权益', rows: equityRows)),
              ],
            )
          else ...[
            AccountGroup(title: '资产', rows: assetRows),
            const SizedBox(height: 8),
            AccountGroup(title: '负债', rows: liabRows),
            const SizedBox(height: 8),
            AccountGroup(title: '权益', rows: equityRows),
          ],
        ];

        // 净资产卡 + 四宫格指标
        final netWorth = ledger.netWorth;
        final totalAssets = ledger.byType['Assets'] ?? {};
        final totalLiabs = ledger.byType['Liabilities'] ?? {};
        final totalIncome = ledger.byType['Income'] ?? {};
        final totalExpense = ledger.byType['Expenses'] ?? {};

        return RefreshIndicator(
          onRefresh: () => ledger.syncNow().then((_) {}),
          child: SingleChildScrollView(
            child: Center(
              child: ConstrainedBox(
                constraints: BoxConstraints(maxWidth: wide ? 1100 : double.infinity),
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      _NetWorthCard(netWorth: netWorth),
                      const SizedBox(height: 12),
                      // 四宫格指标
                      GridView.count(
                        shrinkWrap: true,
                        physics: const NeverScrollableScrollPhysics(),
                        crossAxisCount: wide ? 4 : 2,
                        crossAxisSpacing: 10,
                        mainAxisSpacing: 10,
                        childAspectRatio: 2.4,
                        children: [
                          _MetricChip(label: '总资产', value: _moneyMap(totalAssets), color: AppColors.blue),
                          _MetricChip(label: '总负债', value: _moneyMap(totalLiabs), color: AppColors.warn),
                          _MetricChip(label: '收入', value: _moneyMap(totalIncome), color: AppColors.green),
                          _MetricChip(label: '支出', value: _moneyMap(totalExpense), color: AppColors.warn),
                        ],
                      ),
                      const SizedBox(height: 16),
                      // 近 12 月趋势迷你图
                      _MiniTrendCard(ledger: ledger, wide: wide),
                      const SizedBox(height: 16),
                      ...groups,
                      const SizedBox(height: 16),
                      const SectionTitle('最近交易'),
                      _RecentTxns(ledger: ledger),
                    ],
                  ),
                ),
              ),
            ),
          ),
        );
      },
    );
  }

  LocalAccountRow Function(dynamic) _toRow(LedgerController ledger) =>
      (a) => LocalAccountRow(
            name: a.display,
            subtitle: a.restriction != null ? '限定 ${a.restriction}' : null,
            balances: ledger.balances[a.uuid] ?? {},
          );

  String _moneyMap(Map<String, String> m) {
    if (m.isEmpty) return '—';
    return m.entries.map((e) => formatMoney(e.value, e.key)).join('  ');
  }
}

class _MetricChip extends StatelessWidget {
  final String label;
  final String value;
  final Color color;
  const _MetricChip({required this.label, required this.value, required this.color});

  @override
  Widget build(BuildContext context) => Container(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
        decoration: AppTheme.cardDecoration,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Row(
              children: [
                Container(
                  width: 4,
                  height: 14,
                  decoration: BoxDecoration(
                    color: color,
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
                const SizedBox(width: 6),
                Text(label, style: AppTheme.muted),
              ],
            ),
            const SizedBox(height: 4),
            Text(
              value,
              style: const TextStyle(
                fontSize: 17,
                fontWeight: FontWeight.w700,
                color: AppColors.ink,
                fontFeatures: [FontFeature.tabularFigures()],
              ),
              overflow: TextOverflow.ellipsis,
            ),
          ],
        ),
      );
}

/// 近 12 月收支趋势迷你折线卡。
class _MiniTrendCard extends StatelessWidget {
  final LedgerController ledger;
  final bool wide;
  const _MiniTrendCard({required this.ledger, required this.wide});

  @override
  Widget build(BuildContext context) {
    final accByUuid = {
      for (final a in ledger.accounts) a.uuid: a,
    };
    final now = DateTime.now();
    final points = <TrendPoint>[];

    // 选最大币种
    final totals = <String, Decimal>{};
    for (final t in ledger.txns) {
      for (final p in t.postings) {
        final acc = accByUuid[p.accountUuid];
        if (acc == null) continue;
        final amt = (Decimal.tryParse(p.amount) ?? Decimal.zero).abs();
        totals[p.commodity] = (totals[p.commodity] ?? Decimal.zero) + amt;
      }
    }
    final commodity = totals.isEmpty
        ? 'CNY'
        : totals.entries.reduce((a, b) => a.value >= b.value ? a : b).key;

    for (var i = 11; i >= 0; i--) {
      final m = DateTime(now.year, now.month - i);
      final prefix = '${m.year}-${m.month.toString().padLeft(2, '0')}';
      var inc = Decimal.zero;
      var exp = Decimal.zero;
      for (final t in ledger.txns) {
        if (!t.date.startsWith(prefix)) continue;
        for (final p in t.postings) {
          final acc = accByUuid[p.accountUuid];
          if (acc == null) continue;
          if (p.commodity != commodity) continue;
          final amt = (Decimal.tryParse(p.amount) ?? Decimal.zero).abs();
          if (acc.type == 'Income') {
            inc += amt;
          } else if (acc.type == 'Expenses') {
            exp += amt;
          }
        }
      }
      points.add(TrendPoint(label: '${m.month}月', income: inc, expense: exp));
    }

    return AppCard(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Text('近 12 月收支趋势', style: TextStyle(fontSize: 14, fontWeight: FontWeight.w700)),
              const SizedBox(width: 16),
              _legendDot(AppColors.green, '收入'),
              const SizedBox(width: 12),
              _legendDot(AppColors.warn, '支出'),
              const Spacer(),
              Text('（$commodity）', style: const TextStyle(fontSize: 11, color: AppColors.sub)),
            ],
          ),
          const SizedBox(height: 8),
          TrendLineChart(points: points, commodity: commodity, height: wide ? 200 : 160),
        ],
      ),
    );
  }

  Widget _legendDot(Color c, String label) => Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(width: 8, height: 8, decoration: BoxDecoration(color: c, shape: BoxShape.circle)),
          const SizedBox(width: 4),
          Text(label, style: const TextStyle(fontSize: 11, color: AppColors.sub)),
        ],
      );
}

class _NetWorthCard extends StatelessWidget {
  final Map<String, String> netWorth;
  const _NetWorthCard({required this.netWorth});

  @override
  Widget build(BuildContext context) {
    final entries = netWorth.entries.toList();
    return AppCard(
      padding: const EdgeInsets.all(18),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text('净资产（资产 − 负债）', style: AppTheme.muted),
          const SizedBox(height: 6),
          if (entries.isEmpty)
            const Text('¥ 0.00',
                style: TextStyle(fontSize: 34, fontWeight: FontWeight.w800, letterSpacing: -0.5))
          else
            ...entries.map((e) => Padding(
                  padding: const EdgeInsets.only(bottom: 2),
                  child: Text(
                    formatMoney(e.value, e.key, signed: false),
                    style: const TextStyle(
                      fontSize: 34,
                      fontWeight: FontWeight.w800,
                      letterSpacing: -0.5,
                    ),
                  ),
                )),
          const SizedBox(height: 4),
          const Text('数据来自本地账本，联网后自动同步', style: AppTheme.muted),
        ],
      ),
    );
  }
}

class _RecentTxns extends StatelessWidget {
  final LedgerController ledger;
  final bool hidden; // 首页隐私开关：true 时金额蒙层
  const _RecentTxns({required this.ledger, this.hidden = false});

  @override
  Widget build(BuildContext context) {
    final recent = ledger.txns.take(5).toList();
    if (recent.isEmpty) {
      return const AppCard(child: EmptyState('还没有交易，点右下角 记一笔'));
    }
    return AppCard(
      padding: EdgeInsets.zero,
      child: Column(
        children: [
          for (var i = 0; i < recent.length; i++) ...[
            if (i > 0) const Divider(height: 1, color: AppColors.line),
            _TxnTile(ledger: ledger, txn: recent[i], hidden: hidden),
          ],
        ],
      ),
    );
  }
}

class _TxnTile extends StatelessWidget {
  final LedgerController ledger;
  final LocalTxn txn;
  final bool hidden;
  const _TxnTile({required this.ledger, required this.txn, this.hidden = false});

  @override
  Widget build(BuildContext context) {
    final head = ledger.txnHeadline(txn);
    final headCommodity = txn.postings.isNotEmpty ? txn.postings.first.commodity : 'CNY';
    return ListTile(
      contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 4),
      title: Text(txn.description.isEmpty ? '(无说明)' : txn.description,
          style: const TextStyle(fontWeight: FontWeight.w500)),
      subtitle: Text(
        '${txn.date} · ${ledger.txnFlow(txn)}${txn.createdByName != null && txn.createdByName!.isNotEmpty ? ' · ${txn.createdByName}' : ''}',
        style: AppTheme.muted,
      ),
      trailing: head != null ? AmountText(head, headCommodity, hidden: hidden) : null,
      onTap: () => Navigator.push(
        context,
        MaterialPageRoute(builder: (_) => TxnDetailScreen(txn: txn)),
      ),
    );
  }
}

/// 移动端账单首页（0.4-D）：当月收入/支出/结余大卡 + 月历 + 最近交易 + 眼睛隐私开关。
class _MobileBillHome extends StatefulWidget {
  const _MobileBillHome();

  @override
  State<_MobileBillHome> createState() => _MobileBillHomeState();
}

class _MobileBillHomeState extends State<_MobileBillHome> {
  bool _hide = false;

  @override
  void initState() {
    super.initState();
    _hide = Privacy.load();
  }

  void _toggleHide() {
    setState(() => _hide = !_hide);
    Privacy.save(_hide);
  }

  @override
  Widget build(BuildContext context) {
    final ledger = AppScope.of(context).ledger;
    return ListenableBuilder(
      listenable: ledger,
      builder: (context, _) {
        if (ledger.loading) {
          return const Center(child: CircularProgressIndicator());
        }
        final accByUuid = {
          for (final a in ledger.accounts) a.uuid: a,
        };
        // 选交易总额最大的币种作为本月收支展示口径（避免跨币种直接相加）。
        final totals = <String, Decimal>{};
        for (final t in ledger.txns) {
          for (final p in t.postings) {
            totals[p.commodity] =
                (totals[p.commodity] ?? Decimal.zero) + (Decimal.tryParse(p.amount) ?? Decimal.zero).abs();
          }
        }
        final commodity = totals.isEmpty
            ? 'CNY'
            : totals.entries.reduce((a, b) => a.value >= b.value ? a : b).key;
        final now = DateTime.now();
        final prefix = '${now.year}-${now.month.toString().padLeft(2, '0')}';
        var income = Decimal.zero;
        var expense = Decimal.zero;
        for (final t in ledger.txns) {
          if (!t.date.startsWith(prefix)) continue;
          for (final p in t.postings) {
            if (p.commodity != commodity) continue;
            final acc = accByUuid[p.accountUuid];
            if (acc == null) continue;
            final amt = Decimal.tryParse(p.amount) ?? Decimal.zero;
            if (acc.type == 'Income') {
              income += amt.abs();
            } else if (acc.type == 'Expenses') {
              expense += amt.abs();
            }
          }
        }
        final balance = income - expense;

        return RefreshIndicator(
          onRefresh: () => ledger.syncNow().then((_) {}),
          child: ListView(
            padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
            children: [
              Row(
                children: [
                  const Text('本月账单',
                      style: TextStyle(fontSize: 17, fontWeight: FontWeight.w700)),
                  const Spacer(),
                  IconButton(
                    icon: Icon(_hide ? Icons.visibility_off_outlined : Icons.visibility_outlined),
                    tooltip: _hide ? '显示金额' : '隐藏金额',
                    onPressed: _toggleHide,
                  ),
                ],
              ),
              _MonthSummary(income: income, expense: expense, balance: balance, commodity: commodity, hidden: _hide),
              const SizedBox(height: 16),
              const SectionTitle('当月明细'),
              CalendarView(
                txns: ledger.txns,
                onSelectDate: (date) => Navigator.push(
                  context,
                  MaterialPageRoute(
                    builder: (_) => TransactionsScreen(filterDate: date),
                  ),
                ),
              ),
              const SizedBox(height: 16),
              const SectionTitle('最近交易'),
              _RecentTxns(ledger: ledger, hidden: _hide),
            ],
          ),
        );
      },
    );
  }
}

/// 当月收入/支出/结余大卡。
class _MonthSummary extends StatelessWidget {
  final Decimal income;
  final Decimal expense;
  final Decimal balance;
  final String commodity;
  final bool hidden;
  const _MonthSummary({
    required this.income,
    required this.expense,
    required this.balance,
    required this.commodity,
    required this.hidden,
  });

  String _money(Decimal v) => hidden
      ? '••••'
      : formatMoney(v.toStringAsFixed(2), commodity, signed: false);

  @override
  Widget build(BuildContext context) {
    return AppCard(
      padding: const EdgeInsets.all(18),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text('本月结余（收入 − 支出）', style: AppTheme.muted),
          const SizedBox(height: 6),
          Text(
            _money(balance),
            style: TextStyle(
              fontSize: 34,
              fontWeight: FontWeight.w800,
              letterSpacing: -0.5,
              color: hidden
                  ? AppColors.sub
                  : balance < Decimal.zero
                      ? AppColors.warn
                      : AppColors.ink,
            ),
          ),
          const SizedBox(height: 14),
          Row(
            children: [
              Expanded(child: _Item(label: '收入', value: _money(income), color: AppColors.green)),
              const SizedBox(width: 12),
              Expanded(child: _Item(label: '支出', value: _money(expense), color: AppColors.warn)),
            ],
          ),
        ],
      ),
    );
  }
}

class _Item extends StatelessWidget {
  final String label;
  final String value;
  final Color color;
  const _Item({required this.label, required this.value, required this.color});

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
              fontSize: 17,
              fontWeight: FontWeight.w700,
              fontFeatures: [FontFeature.tabularFigures()],
            ),
          ),
        ],
      );
}
