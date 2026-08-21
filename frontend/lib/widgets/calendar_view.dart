/// 月历视图（0.4-D）：按天显示当日净值，点击某天跳转当日流水。
/// 选择交易总额最大的币种作为展示口径；当日净值 = 当日所有分录金额（有符号）之和。
import 'package:decimal/decimal.dart';
import 'package:flutter/material.dart';
import '../app/theme.dart';
import '../core/models/transaction.dart';
import '../utils/money.dart';
import 'common.dart';

class CalendarView extends StatefulWidget {
  /// 已（按账户/标签/搜索等）过滤后的交易列表。
  final List<LocalTxn> txns;
  /// 点击某天（含交易的日期）回调，参数为 yyyy-MM-dd。
  final void Function(String date) onSelectDate;
  const CalendarView({super.key, required this.txns, required this.onSelectDate});

  @override
  State<CalendarView> createState() => _CalendarViewState();
}

class _CalendarViewState extends State<CalendarView> {
  late DateTime _cursor;

  @override
  void initState() {
    super.initState();
    _cursor = DateTime.now();
  }

  String get _prefix =>
      '${_cursor.year}-${_cursor.month.toString().padLeft(2, '0')}';

  /// 选择总额最大的币种作为展示口径。
  String get _dominant {
    final totals = <String, Decimal>{};
    for (final t in widget.txns) {
      for (final p in t.postings) {
        totals[p.commodity] =
            (totals[p.commodity] ?? Decimal.zero) + (Decimal.tryParse(p.amount) ?? Decimal.zero).abs();
      }
    }
    if (totals.isEmpty) return 'CNY';
    return totals.entries.reduce((a, b) => a.value >= b.value ? a : b).key;
  }

  /// 本月内每日净值（有符号）。
  Map<String, Decimal> get _dayNet {
    final m = <String, Decimal>{};
    for (final t in widget.txns) {
      if (!t.date.startsWith(_prefix)) continue;
      for (final p in t.postings) {
        if (p.commodity != _dominant) continue;
        m[t.date] = (m[t.date] ?? Decimal.zero) + (Decimal.tryParse(p.amount) ?? Decimal.zero);
      }
    }
    return m;
  }

  /// 本月内有交易记录的日期集合。
  Set<String> get _daysWithTxn =>
      {for (final t in widget.txns) if (t.date.startsWith(_prefix)) t.date};

  void _shift(int months) {
    setState(() {
      _cursor = DateTime(_cursor.year, _cursor.month + months);
    });
  }

  @override
  Widget build(BuildContext context) {
    final dayNet = _dayNet;
    final daysWithTxn = _daysWithTxn;
    final first = DateTime(_cursor.year, _cursor.month, 1);
    final leading = (first.weekday - 1) % 7; // 周一为首列
    final daysInMonth = DateTime(_cursor.year, _cursor.month + 1, 0).day;
    final cells = leading + daysInMonth;
    final total = cells + (7 - cells % 7) % 7;

    final todayStr =
        '${DateTime.now().year}-${DateTime.now().month.toString().padLeft(2, '0')}-${DateTime.now().day.toString().padLeft(2, '0')}';

    return AppCard(
      padding: const EdgeInsets.all(14),
      child: Column(
        children: [
          Row(
            children: [
              IconButton(
                icon: const Icon(Icons.chevron_left, size: 20),
                onPressed: () => _shift(-1),
                tooltip: '上一月',
                visualDensity: VisualDensity.compact,
              ),
              Expanded(
                child: Text(
                  '${_cursor.year}年${_cursor.month}月',
                  textAlign: TextAlign.center,
                  style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w700),
                ),
              ),
              IconButton(
                icon: const Icon(Icons.chevron_right, size: 20),
                onPressed: () => _shift(1),
                tooltip: '下一月',
                visualDensity: VisualDensity.compact,
              ),
            ],
          ),
          const SizedBox(height: 8),
          Row(
            children: const ['一', '二', '三', '四', '五', '六', '日']
                .map((w) => Expanded(
                      child: Center(
                        child: Text(w, style: TextStyle(fontSize: 12, color: AppColors.sub)),
                      ),
                    ))
                .toList(),
          ),
          const SizedBox(height: 4),
          GridView.count(
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            crossAxisCount: 7,
            childAspectRatio: 1.15,
            children: [
              for (var i = 0; i < total; i++)
                if (i < leading || i >= leading + daysInMonth)
                  const SizedBox.shrink()
                else
                  _DayCell(
                    day: i - leading + 1,
                    dateStr:
                        '$_prefix-${(i - leading + 1).toString().padLeft(2, '0')}',
                    net: dayNet['$_prefix-${(i - leading + 1).toString().padLeft(2, '0')}'],
                    hasTxn: daysWithTxn.contains(
                        '$_prefix-${(i - leading + 1).toString().padLeft(2, '0')}'),
                    commodity: _dominant,
                    isToday: todayStr ==
                        '$_prefix-${(i - leading + 1).toString().padLeft(2, '0')}',
                    onSelect: widget.onSelectDate,
                  ),
            ],
          ),
        ],
      ),
    );
  }
}

class _DayCell extends StatelessWidget {
  final int day;
  final String dateStr;
  final Decimal? net;
  final bool hasTxn; // 当日是否有交易（独立于净值，净值为 0 也可点击）
  final String commodity;
  final bool isToday;
  final void Function(String date) onSelect;
  const _DayCell({
    required this.day,
    required this.dateStr,
    required this.net,
    required this.hasTxn,
    required this.commodity,
    required this.isToday,
    required this.onSelect,
  });

  @override
  Widget build(BuildContext context) {
    final color = net == null
        ? AppColors.sub
        : net! < Decimal.zero
            ? AppColors.warn
            : AppColors.green;
    final child = Container(
      margin: const EdgeInsets.all(2),
      decoration: BoxDecoration(
        color: isToday ? AppColors.blue.withOpacity(0.12) : null,
        borderRadius: BorderRadius.circular(8),
        border: hasTxn ? Border.all(color: color.withOpacity(0.35)) : null,
      ),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Text('$day',
              style: TextStyle(
                fontSize: 13,
                fontWeight: isToday ? FontWeight.w700 : FontWeight.w500,
                color: isToday ? AppColors.blue : AppColors.ink,
              )),
          if (hasTxn)
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 2),
              child: Text(
                _fmt(net ?? Decimal.zero, commodity),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(fontSize: 9.5, color: color, fontWeight: FontWeight.w600),
              ),
            ),
        ],
      ),
    );
    if (!hasTxn) return child;
    return InkWell(onTap: () => onSelect(dateStr), child: child);
  }

  String _fmt(Decimal v, String c) {
    final s = formatMoney(v.toStringAsFixed(2), c, signed: false);
    // 紧凑展示：去掉币种符号与千分位前缀，仅留数值
    final idx = s.indexOf(RegExp(r'[0-9]'));
    return idx >= 0 ? s.substring(idx) : s;
  }
}
