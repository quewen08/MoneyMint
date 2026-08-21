/// PC 端「报表分析」：基于本地交易数据计算。
/// 0.4-B 升级：饼图占比 + 收支双线趋势 + 分类排行 + 周/月/年时间筛选。
import 'package:decimal/decimal.dart';
import 'package:flutter/material.dart';
import '../../app/navigation.dart';
import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../core/models/account.dart';
import '../../core/models/transaction.dart';
import '../../utils/money.dart';
import '../../widgets/charts.dart';
import '../../widgets/common.dart';
import '../../widgets/pc.dart';

/// 时间筛选口径。
enum TrendRange { week, month, year }

/// 临时筛选范围（基于当前月/周的起止日期串）。
typedef DateRangeFilter = bool Function(String date);

class ReportsScreen extends StatefulWidget {
  const ReportsScreen({super.key});

  @override
  State<ReportsScreen> createState() => _ReportsScreenState();
}

class _ReportsScreenState extends State<ReportsScreen> {
  TrendRange _range = TrendRange.month;
  DateTime _cursor = DateTime.now();
  String? _tag; // 标签下钻（§5）：null=全部

  @override
  Widget build(BuildContext context) {
    final ledger = AppScope.of(context).ledger;
    return ListenableBuilder(
      listenable: ledger,
      builder: (context, _) {
        final wide = isWide(context);
        final filter = _buildFilter();
        final allTags = <String>{
          for (final t in ledger.txns) ...t.tags,
        }.toList()
          ..sort();
        final txns = ledger.txns
            .where((t) => filter(t.date) && (_tag == null || t.tags.contains(_tag)))
            .toList();
        return SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 1100),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  PcSectionTitle('核心指标', actions: _rangeActions()),
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
                        value: '${txns.length}',
                      ),
                    ],
                  ),
                  if (allTags.isNotEmpty) ...[
                    const SizedBox(height: 16),
                    PcSectionTitle('标签下钻'),
                    const SizedBox(height: 8),
                    Wrap(
                      spacing: 6,
                      runSpacing: 4,
                      children: [
                        ChoiceChip(
                          label: const Text('全部', style: TextStyle(fontSize: 12)),
                          selected: _tag == null,
                          onSelected: (_) => setState(() => _tag = null),
                        ),
                        for (final tg in allTags)
                          ChoiceChip(
                            label: Text('#$tg', style: const TextStyle(fontSize: 12)),
                            selected: _tag == tg,
                            onSelected: (_) => setState(() => _tag = tg),
                          ),
                      ],
                    ),
                  ],
                  const SizedBox(height: 24),
                  // 收支双线趋势
                  PcSectionTitle('收支趋势'),
                  const SizedBox(height: 12),
                  _TrendCard(ledger: ledger, txns: txns, range: _range),
                  const SizedBox(height: 24),
                  // 饼图 + 排行
                  wide
                      ? Row(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Expanded(child: _ExpensePieCard(ledger: ledger, txns: txns)),
                            const SizedBox(width: 16),
                            Expanded(child: _RankCard(ledger: ledger, txns: txns)),
                          ],
                        )
                      : Column(
                          children: [
                            _ExpensePieCard(ledger: ledger, txns: txns),
                            const SizedBox(height: 16),
                            _RankCard(ledger: ledger, txns: txns),
                          ],
                        ),
                  const SizedBox(height: 24),
                  // 试算平衡
                  PcSectionTitle('试算平衡'),
                  const SizedBox(height: 12),
                  _TrialBalance(ledger: ledger),
                ],
              ),
            ),
          ),
        );
      },
    );
  }

  List<Widget> _rangeActions() {
    const labels = ['周', '月', '年'];
    const values = [TrendRange.week, TrendRange.month, TrendRange.year];
    return [
      for (var i = 0; i < labels.length; i++)
        ChoiceChip(
          label: Text(labels[i]),
          selected: _range == values[i],
          onSelected: (_) => setState(() {
            _range = values[i];
            _cursor = DateTime.now();
          }),
        ),
      IconButton(
        icon: const Icon(Icons.chevron_left, size: 20),
        onPressed: () => setState(() => _cursor = _prevCursor()),
        tooltip: '上一$_rangeLabel',
      ),
      Text(_cursorLabel(), style: const TextStyle(fontSize: 13, color: AppColors.sub)),
      IconButton(
        icon: const Icon(Icons.chevron_right, size: 20),
        onPressed: () => setState(() => _cursor = _nextCursor()),
        tooltip: '下一$_rangeLabel',
      ),
    ];
  }

  String get _rangeLabel {
    switch (_range) {
      case TrendRange.week:
        return '周';
      case TrendRange.month:
        return '月';
      case TrendRange.year:
        return '年';
    }
  }

  DateTime _prevCursor() {
    switch (_range) {
      case TrendRange.week:
        return _cursor.subtract(const Duration(days: 7));
      case TrendRange.month:
        return DateTime(_cursor.year, _cursor.month - 1);
      case TrendRange.year:
        return DateTime(_cursor.year - 1);
    }
  }

  DateTime _nextCursor() {
    switch (_range) {
      case TrendRange.week:
        return _cursor.add(const Duration(days: 7));
      case TrendRange.month:
        return DateTime(_cursor.year, _cursor.month + 1);
      case TrendRange.year:
        return DateTime(_cursor.year + 1);
    }
  }

  String _cursorLabel() {
    switch (_range) {
      case TrendRange.week:
        final start = _cursor.subtract(Duration(days: _cursor.weekday - 1));
        final end = start.add(const Duration(days: 6));
        return '${todayStr(start)} ~ ${todayStr(end)}';
      case TrendRange.month:
        return '${_cursor.year}-${_cursor.month.toString().padLeft(2, '0')}';
      case TrendRange.year:
        return '${_cursor.year}';
    }
  }

  DateRangeFilter _buildFilter() {
    switch (_range) {
      case TrendRange.week:
        final start = _cursor.subtract(Duration(days: _cursor.weekday - 1));
        final startStr = todayStr(start);
        final endStr = todayStr(start.add(const Duration(days: 6)));
        return (d) => d.compareTo(startStr) >= 0 && d.compareTo(endStr) <= 0;
      case TrendRange.month:
        final prefix = '${_cursor.year}-${_cursor.month.toString().padLeft(2, '0')}';
        return (d) => d.startsWith(prefix);
      case TrendRange.year:
        final prefix = '${_cursor.year}';
        return (d) => d.startsWith(prefix);
    }
  }

  String _moneyMap(Map<String, String> m) {
    if (m.isEmpty) return '—';
    final lines = m.entries.map((e) => formatMoney(e.value, e.key)).toList();
    return lines.join('  ');
  }
}

/// 收支趋势卡：按时间段聚合，展示双线图（收入 vs 支出）。
class _TrendCard extends StatelessWidget {
  final dynamic ledger;
  final List<LocalTxn> txns;
  final TrendRange range;
  const _TrendCard({required this.ledger, required this.txns, required this.range});

  @override
  Widget build(BuildContext context) {
    final accByUuid = {
      for (final a in ledger.accounts as List) a.uuid as String: a as LocalAccount,
    };

    // 选择总额最大的币种作为展示口径
    final commodity = _pickCommodity(txns, accByUuid);

    // 按时间段聚合
    final points = <TrendPoint>[];
    if (range == TrendRange.week) {
      // 本周 7 天
      final today = DateTime.now();
      final start = today.subtract(Duration(days: today.weekday - 1));
      for (var i = 0; i < 7; i++) {
        final day = start.add(Duration(days: i));
        final dayStr = todayStr(day);
        final inc = _sumByType(txns, accByUuid, commodity, 'Income', (d) => d == dayStr);
        final exp = _sumByType(txns, accByUuid, commodity, 'Expenses', (d) => d == dayStr);
        points.add(TrendPoint(label: '周${_weekdayName(day.weekday)}', income: inc, expense: exp));
      }
    } else if (range == TrendRange.month) {
      // 本月按天聚合
      final now = DateTime.now();
      final daysInMonth = DateTime(now.year, now.month + 1, 0).day;
      for (var d = 1; d <= daysInMonth; d++) {
        final dayStr = '${now.year}-${now.month.toString().padLeft(2, '0')}-${d.toString().padLeft(2, '0')}';
        final label = '${now.month.toString().padLeft(2, '0')}/${d.toString().padLeft(2, '0')}';
        final inc = _sumByType(txns, accByUuid, commodity, 'Income', (date) => date == dayStr);
        final exp = _sumByType(txns, accByUuid, commodity, 'Expenses', (date) => date == dayStr);
        points.add(TrendPoint(label: label, income: inc, expense: exp));
      }
    } else {
      // 本年 12 个月
      final now = DateTime.now();
      for (var m = 1; m <= 12; m++) {
        final prefix = '${now.year}-${m.toString().padLeft(2, '0')}';
        final inc = _sumByType(txns, accByUuid, commodity, 'Income', (d) => d.startsWith(prefix));
        final exp = _sumByType(txns, accByUuid, commodity, 'Expenses', (d) => d.startsWith(prefix));
        points.add(TrendPoint(label: '$m月', income: inc, expense: exp));
      }
    }

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: AppTheme.cardDecoration,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Text('收支趋势', style: TextStyle(fontSize: 12, color: AppColors.sub)),
              const SizedBox(width: 16),
              _legendDot(AppColors.green, '收入'),
              const SizedBox(width: 12),
              _legendDot(AppColors.warn, '支出'),
              const Spacer(),
              Text('（$commodity）', style: const TextStyle(fontSize: 11, color: AppColors.sub)),
            ],
          ),
          const SizedBox(height: 8),
          TrendLineChart(points: points, commodity: commodity),
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

  String _weekdayName(int w) {
    const names = ['', '一', '二', '三', '四', '五', '六', '日'];
    return names[w];
  }
}

/// 支出分类饼图卡。
class _ExpensePieCard extends StatelessWidget {
  final dynamic ledger;
  final List<LocalTxn> txns;
  const _ExpensePieCard({required this.ledger, required this.txns});

  @override
  Widget build(BuildContext context) {
    final accByUuid = {
      for (final a in ledger.accounts as List) a.uuid as String: a as LocalAccount,
    };
    final commodity = _pickCommodity(txns, accByUuid);
    final sum = <String, Decimal>{};
    for (final t in txns) {
      for (final p in t.postings) {
        final acc = accByUuid[p.accountUuid];
        if (acc?.type != 'Expenses') continue;
        if (p.commodity != commodity) continue;
        final amt = (Decimal.tryParse(p.amount) ?? Decimal.zero).abs();
        sum[acc!.display] = (sum[acc.display] ?? Decimal.zero) + amt;
      }
    }
    final entries = sum.entries.toList()
      ..sort((a, b) => b.value.compareTo(a.value));
    final slices = <PieSlice>[];
    const palette = [AppColors.blue, AppColors.green, AppColors.warn, Color(0xFF8B5CF6), Color(0xFFEC4899), Color(0xFF14B8A6)];
    for (var i = 0; i < entries.length && i < 8; i++) {
      slices.add(PieSlice(
        label: entries[i].key,
        value: entries[i].value,
        color: palette[i % palette.length],
      ));
    }
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: AppTheme.cardDecoration,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text('支出分类占比', style: TextStyle(fontSize: 14, fontWeight: FontWeight.w700)),
          const SizedBox(height: 12),
          PieShareChart(slices: slices, commodity: commodity),
        ],
      ),
    );
  }
}

/// 支出分类排行卡。
class _RankCard extends StatelessWidget {
  final dynamic ledger;
  final List<LocalTxn> txns;
  const _RankCard({required this.ledger, required this.txns});

  @override
  Widget build(BuildContext context) {
    final accByUuid = {
      for (final a in ledger.accounts as List) a.uuid as String: a as LocalAccount,
    };
    final commodity = _pickCommodity(txns, accByUuid);
    final sum = <String, Decimal>{};
    for (final t in txns) {
      for (final p in t.postings) {
        final acc = accByUuid[p.accountUuid];
        if (acc?.type != 'Expenses') continue;
        if (p.commodity != commodity) continue;
        final amt = (Decimal.tryParse(p.amount) ?? Decimal.zero).abs();
        sum[acc!.display] = (sum[acc.display] ?? Decimal.zero) + amt;
      }
    }
    final entries = sum.entries.toList()
      ..sort((a, b) => b.value.compareTo(a.value));
    const palette = [AppColors.blue, AppColors.green, AppColors.warn, Color(0xFF8B5CF6), Color(0xFFEC4899), Color(0xFF14B8A6)];
    final items = <PieSlice>[];
    for (var i = 0; i < entries.length && i < 10; i++) {
      items.add(PieSlice(
        label: entries[i].key,
        value: entries[i].value,
        color: palette[i % palette.length],
      ));
    }
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: AppTheme.cardDecoration,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text('支出排行', style: TextStyle(fontSize: 14, fontWeight: FontWeight.w700)),
          const SizedBox(height: 12),
          RankBarChart(items: items, commodity: commodity),
        ],
      ),
    );
  }
}

/// 选择支出总额最大的币种作为展示口径。
String _pickCommodity(List<LocalTxn> txns, Map<String, LocalAccount> accByUuid) {
  final totals = <String, Decimal>{};
  for (final t in txns) {
    for (final p in t.postings) {
      final acc = accByUuid[p.accountUuid];
      if (acc?.type != 'Expenses') continue;
      final amt = (Decimal.tryParse(p.amount) ?? Decimal.zero).abs();
      totals[p.commodity] = (totals[p.commodity] ?? Decimal.zero) + amt;
    }
  }
  if (totals.isEmpty) return 'CNY';
  return totals.entries.reduce((a, b) => a.value >= b.value ? a : b).key;
}

Decimal _sumByType(
  List<LocalTxn> txns,
  Map<String, LocalAccount> accByUuid,
  String commodity,
  String type,
  bool Function(String date) dateFilter,
) {
  var sum = Decimal.zero;
  for (final t in txns) {
    if (!dateFilter(t.date)) continue;
    for (final p in t.postings) {
      final acc = accByUuid[p.accountUuid];
      if (acc?.type != type) continue;
      if (p.commodity != commodity) continue;
      sum += (Decimal.tryParse(p.amount) ?? Decimal.zero).abs();
    }
  }
  return sum;
}

class _TrialBalance extends StatelessWidget {
  final dynamic ledger;
  const _TrialBalance({required this.ledger});

  @override
  Widget build(BuildContext context) {
    final rows = <Map<String, dynamic>>[];
    for (final a in ledger.accounts as List) {
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
