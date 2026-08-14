/// PC 端「报表分析」：完全基于本地交易数据计算。
/// 含试算平衡（账户余额）、支出分类汇总、月度支出趋势。
import 'package:decimal/decimal.dart';
import 'package:flutter/material.dart';
import '../../app/navigation.dart';
import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../utils/money.dart';
import '../../widgets/pc.dart';

class ReportsScreen extends StatelessWidget {
  const ReportsScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final ledger = AppScope.of(context).ledger;
    return ListenableBuilder(
      listenable: ledger,
      builder: (context, _) {
        final wide = isWide(context);
        return SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 1100),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  PcSectionTitle('核心指标'),
                  const SizedBox(height: 12),
                  GridView.count(
                    shrinkWrap: true,
                    physics: const NeverScrollableScrollPhysics(),
                    crossAxisCount: wide ? 4 : 2,
                    crossAxisSpacing: 12,
                    mainAxisSpacing: 12,
                    childAspectRatio: 2.2,
                    children: [
                      PcMetricCard(
                        label: '净资产',
                        value: _moneyMap(ledger.netWorth),
                      ),
                      PcMetricCard(
                        label: '总资产',
                        value: _moneyMap(ledger.byType['Assets'] ?? {}),
                      ),
                      PcMetricCard(
                        label: '总负债',
                        value: _moneyMap(ledger.byType['Liabilities'] ?? {}),
                      ),
                      PcMetricCard(
                        label: '交易笔数',
                        value: '${ledger.txns.length}',
                      ),
                    ],
                  ),
                  const SizedBox(height: 24),
                  PcSectionTitle('试算平衡'),
                  const SizedBox(height: 12),
                  _TrialBalance(ledger: ledger),
                  const SizedBox(height: 24),
                  PcSectionTitle('支出分类'),
                  const SizedBox(height: 12),
                  _ExpenseByCategory(ledger: ledger),
                  const SizedBox(height: 24),
                  PcSectionTitle('月度支出趋势'),
                  const SizedBox(height: 12),
                  _MonthlyTrend(ledger: ledger),
                ],
              ),
            ),
          ),
        );
      },
    );
  }

  String _moneyMap(Map<String, String> m) {
    if (m.isEmpty) return '—';
    final lines = m.entries.map((e) => formatMoney(e.value, e.key)).toList();
    return lines.join('  ');
  }
}

class _TrialBalance extends StatelessWidget {
  final dynamic ledger;
  const _TrialBalance({required this.ledger});

  @override
  Widget build(BuildContext context) {
    final rows = <Map<String, dynamic>>[];
    for (final a in ledger.accounts) {
      final bal = ledger.balances[a.uuid] as Map<String, String>? ?? {};
      if (bal.isEmpty) continue;
      bal.forEach((commodity, amount) {
        rows.add({'name': a.display, 'type': a.type, 'commodity': commodity, 'amount': amount});
      });
    }
    if (rows.isEmpty) {
      return Container(
        padding: const EdgeInsets.all(16),
        decoration: AppTheme.cardDecoration,
        child: const Text('暂无余额数据，先记几笔吧。',
            style: TextStyle(color: AppColors.sub)),
      );
    }
    return Container(
      decoration: AppTheme.cardDecoration,
      child: Table(
        columnWidths: const {
          0: FlexColumnWidth(3),
          1: FlexColumnWidth(1.2),
          2: FlexColumnWidth(2),
        },
        children: [
          TableRow(
            decoration: const BoxDecoration(color: Color(0xFFF5F6F8)),
            children: const [
              Padding(padding: EdgeInsets.all(10), child: Text('账户', style: TextStyle(fontWeight: FontWeight.w700, fontSize: 12))),
              Padding(padding: EdgeInsets.all(10), child: Text('类型', style: TextStyle(fontWeight: FontWeight.w700, fontSize: 12))),
              Padding(padding: EdgeInsets.all(10), child: Text('余额', style: TextStyle(fontWeight: FontWeight.w700, fontSize: 12), textAlign: TextAlign.right)),
            ],
          ),
          for (final r in rows)
            TableRow(
              children: [
                Padding(padding: const EdgeInsets.all(10), child: Text(r['name'], style: const TextStyle(fontSize: 13))),
                Padding(padding: const EdgeInsets.all(10), child: Text(r['type'], style: const TextStyle(fontSize: 12, color: AppColors.sub))),
                Padding(
                  padding: const EdgeInsets.all(10),
                  child: Text(formatMoney(r['amount'], r['commodity']),
                      style: const TextStyle(fontSize: 13, fontFeatures: [FontFeature.tabularFigures()]),
                      textAlign: TextAlign.right),
                ),
              ],
            ),
        ],
      ),
    );
  }
}

class _ExpenseByCategory extends StatelessWidget {
  final dynamic ledger;
  const _ExpenseByCategory({required this.ledger});

  @override
  Widget build(BuildContext context) {
    final accByUuid = {
      for (final a in ledger.accounts) a.uuid: a
    };
    final Map<String, Map<String, Decimal>> sum = {};
    for (final t in ledger.txns) {
      for (final p in t.postings) {
        final acc = accByUuid[p.accountUuid];
        if (acc?.type != 'Expenses') continue;
        final amt = (Decimal.tryParse(p.amount) ?? Decimal.zero).abs();
        sum.putIfAbsent(acc!.display, () => {})[p.commodity] =
            (sum[acc.display]?[p.commodity] ?? Decimal.zero) + amt;
      }
    }
    final entries = sum.entries.toList()
      ..sort((a, b) {
        final ta = a.value.values.fold(Decimal.zero, (x, y) => x + y);
        final tb = b.value.values.fold(Decimal.zero, (x, y) => x + y);
        return tb.compareTo(ta);
      });
    if (entries.isEmpty) {
      return Container(
        padding: const EdgeInsets.all(16),
        decoration: AppTheme.cardDecoration,
        child: const Text('暂无支出数据。', style: TextStyle(color: AppColors.sub)),
      );
    }
    return Container(
      decoration: AppTheme.cardDecoration,
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Column(
        children: [
          for (final e in entries.take(10))
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
              child: Row(
                children: [
                  Expanded(child: Text(e.key, style: const TextStyle(fontSize: 13))),
                  Text(
                    e.value.entries
                        .map((v) => formatMoney(v.value.toString(), v.key))
                        .join('  '),
                    style: const TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w600,
                        color: AppColors.warn,
                        fontFeatures: [FontFeature.tabularFigures()]),
                  ),
                ],
              ),
            ),
        ],
      ),
    );
  }
}

class _MonthlyTrend extends StatelessWidget {
  final dynamic ledger;
  const _MonthlyTrend({required this.ledger});

  @override
  Widget build(BuildContext context) {
    final accByUuid = {
      for (final a in ledger.accounts) a.uuid: a
    };
    final Map<String, Map<String, Decimal>> byMonth = {};
    for (final t in ledger.txns) {
      final month = t.date.length >= 7 ? t.date.substring(0, 7) : t.date;
      for (final p in t.postings) {
        final acc = accByUuid[p.accountUuid];
        if (acc?.type != 'Expenses') continue;
        final amt = (Decimal.tryParse(p.amount) ?? Decimal.zero).abs();
        byMonth.putIfAbsent(month, () => {})[p.commodity] =
            (byMonth[month]?[p.commodity] ?? Decimal.zero) + amt;
      }
    }
    if (byMonth.isEmpty) {
      return Container(
        padding: const EdgeInsets.all(16),
        decoration: AppTheme.cardDecoration,
        child: const Text('暂无支出数据。', style: TextStyle(color: AppColors.sub)),
      );
    }
    // 选择支出总额最大的币种作为趋势展示口径。
    final totals = <String, Decimal>{};
    byMonth.forEach((_, m) => m.forEach((c, v) =>
        totals[c] = (totals[c] ?? Decimal.zero) + v));
    final commodity = totals.entries.isEmpty
        ? ''
        : totals.entries.reduce((a, b) => a.value >= b.value ? a : b).key;

    final months = byMonth.keys.toList()..sort();
    final values = months
        .map((m) => byMonth[m]?[commodity] ?? Decimal.zero)
        .toList();
    final maxV = values.fold(Decimal.zero, (a, b) => a > b ? a : b);
    final ratios = values
        .map((v) => maxV == Decimal.zero
            ? 0.0
            : (v / maxV).toDouble())
        .toList();
    final colors = List.generate(
        months.length, (_) => AppColors.blue);

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: AppTheme.cardDecoration,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('支出趋势（$commodity）',
              style: const TextStyle(fontSize: 12, color: AppColors.sub)),
          const SizedBox(height: 10),
          MiniBars(
            ratios: ratios,
            colors: colors,
            labels: months.map((m) => m.substring(5)).toList(),
          ),
        ],
      ),
    );
  }
}
