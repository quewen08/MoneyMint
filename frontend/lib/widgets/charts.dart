/// 0.4-B 图表组件库：封装 fl_chart 的饼图、折线趋势、条形排行。
/// 所有金额计算仍用 Decimal 定点十进制，仅在传给 fl_chart 时转 double（展示用，不参与运算）。
library;

import 'package:decimal/decimal.dart';
import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';
import '../app/theme.dart';
import '../utils/money.dart';

/// 饼图数据项。
class PieSlice {
  final String label;
  final Decimal value;
  final Color color;
  const PieSlice({
    required this.label,
    required this.value,
    required this.color,
  });
}

/// 分类占比饼图（支出/收入分类用）。
/// [slices] 已按降序排好；[commodity] 用于 tooltip 金额格式化。
class PieShareChart extends StatelessWidget {
  final List<PieSlice> slices;
  final String commodity;
  final double size;
  const PieShareChart({
    super.key,
    required this.slices,
    required this.commodity,
    this.size = 200,
  });

  static const _palette = [
    AppColors.blue,
    AppColors.green,
    AppColors.warn,
    Color(0xFF8B5CF6),
    Color(0xFFEC4899),
    Color(0xFF14B8A6),
    Color(0xFFF59E0B),
    Color(0xFF6366F1),
  ];

  @override
  Widget build(BuildContext context) {
    if (slices.isEmpty) {
      return _empty('暂无数据');
    }
    final total = slices.fold(Decimal.zero, (s, e) => s + e.value);
    final sections = <PieChartSectionData>[];
    for (var i = 0; i < slices.length; i++) {
      final s = slices[i];
      final pct = total == Decimal.zero
          ? 0.0
          : (s.value / total).toDouble();
      sections.add(PieChartSectionData(
        value: s.value.toDouble(),
        color: _palette[i % _palette.length],
        radius: size * 0.18,
        title: pct >= 0.08 ? '${(pct * 100).toStringAsFixed(0)}%' : '',
        titleStyle: const TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w700,
          color: Colors.white,
        ),
      ));
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          height: size,
          child: PieChart(PieChartData(
            sections: sections,
            centerSpaceRadius: size * 0.22,
            sectionsSpace: 2,
          )),
        ),
        const SizedBox(height: 12),
        Wrap(
          spacing: 16,
          runSpacing: 8,
          children: [
            for (var i = 0; i < slices.length; i++)
              _Legend(
                color: _palette[i % _palette.length],
                label: slices[i].label,
                amount: formatMoney(slices[i].value.toString(), commodity),
              ),
          ],
        ),
      ],
    );
  }

  Widget _empty(String msg) => SizedBox(
        height: size,
        child: Center(
          child: Text(msg, style: const TextStyle(color: AppColors.sub)),
        ),
      );
}

class _Legend extends StatelessWidget {
  final Color color;
  final String label;
  final String amount;
  const _Legend({
    required this.color,
    required this.label,
    required this.amount,
  });

  @override
  Widget build(BuildContext context) => Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 10,
            height: 10,
            decoration: BoxDecoration(color: color, shape: BoxShape.circle),
          ),
          const SizedBox(width: 6),
          Text(label, style: const TextStyle(fontSize: 12, color: AppColors.ink)),
          const SizedBox(width: 4),
          Text(amount,
              style: const TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w600,
                  color: AppColors.sub,
                  fontFeatures: [FontFeature.tabularFigures()])),
        ],
      );
}

/// 趋势折线图数据点。
class TrendPoint {
  final String label; // x 轴标签（如 "01" 表示 1 月）
  final Decimal income;
  final Decimal expense;
  const TrendPoint({
    required this.label,
    required this.income,
    required this.expense,
  });
}

/// 收支双线趋势图（月度/周度/年度用）。
class TrendLineChart extends StatelessWidget {
  final List<TrendPoint> points;
  final String commodity;
  final double height;
  const TrendLineChart({
    super.key,
    required this.points,
    required this.commodity,
    this.height = 220,
  });

  @override
  Widget build(BuildContext context) {
    if (points.isEmpty) {
      return _emptyBox('暂无趋势数据');
    }
    final allValues = <double>[
      for (final p in points) ...[p.income.toDouble(), p.expense.toDouble()],
    ];
    final maxY = allValues.reduce((a, b) => a > b ? a : b) * 1.15;
    final minY = 0.0;

    return SizedBox(
      height: height,
      child: Padding(
        padding: const EdgeInsets.only(right: 12, top: 8),
        child: LineChart(
          LineChartData(
            gridData: const FlGridData(
              show: true,
              drawVerticalLine: false,
              horizontalInterval: null,
              getDrawingHorizontalLine: _hGrid,
            ),
            titlesData: FlTitlesData(
              topTitles: const AxisTitles(sideTitles: SideTitles(showTitles: false)),
              rightTitles: const AxisTitles(sideTitles: SideTitles(showTitles: false)),
              leftTitles: AxisTitles(
                sideTitles: SideTitles(
                  showTitles: true,
                  reservedSize: 48,
                  getTitlesWidget: (v, _) => Text(
                    _compactMoney(v, commodity),
                    style: const TextStyle(fontSize: 10, color: AppColors.sub),
                  ),
                ),
              ),
              bottomTitles: AxisTitles(
                sideTitles: SideTitles(
                  showTitles: true,
                  interval: points.length > 8 ? (points.length / 6).ceilToDouble() : 1,
                  getTitlesWidget: (v, _) {
                    final i = v.round();
                    if (i < 0 || i >= points.length) return const SizedBox();
                    return Padding(
                      padding: const EdgeInsets.only(top: 4),
                      child: Text(points[i].label,
                          style: const TextStyle(fontSize: 10, color: AppColors.sub)),
                    );
                  },
                ),
              ),
            ),
            borderData: FlBorderData(show: false),
            minY: minY,
            maxY: maxY == 0 ? 1.0 : maxY,
            lineBarsData: [
              _line(
                points.map((p) => FlSpot(points.indexOf(p).toDouble(), p.income.toDouble())).toList(),
                AppColors.green,
              ),
              _line(
                points.map((p) => FlSpot(points.indexOf(p).toDouble(), p.expense.toDouble())).toList(),
                AppColors.warn,
              ),
            ],
            lineTouchData: LineTouchData(
              touchTooltipData: LineTouchTooltipData(
                getTooltipItems: (spots) {
                  final idx = spots.first.spotIndex;
                  if (idx < 0 || idx >= points.length) {
                    return <LineTooltipItem>[];
                  }
                  final p = points[idx];
                  return [
                    LineTooltipItem(
                      '收 ${formatMoney(p.income.toString(), commodity)}',
                      const TextStyle(fontSize: 11, color: AppColors.green, fontWeight: FontWeight.w600),
                    ),
                    LineTooltipItem(
                      '支 ${formatMoney(p.expense.toString(), commodity)}',
                      const TextStyle(fontSize: 11, color: AppColors.warn, fontWeight: FontWeight.w600),
                    ),
                  ];
                },
              ),
            ),
          ),
        ),
      ),
    );
  }

  LineChartBarData _line(List<FlSpot> spots, Color color) => LineChartBarData(
        spots: spots,
        isCurved: true,
        color: color,
        barWidth: 2.5,
        dotData: const FlDotData(show: false),
        belowBarData: BarAreaData(
          show: true,
          color: color.withOpacity(0.08),
        ),
      );

  Widget _emptyBox(String msg) => SizedBox(
        height: height,
        child: Center(
          child: Text(msg, style: const TextStyle(color: AppColors.sub)),
        ),
      );
}

FlLine _hGrid(double v) => const FlLine(
      color: AppColors.line,
      strokeWidth: 0.5,
      dashArray: [4, 4],
    );

/// 分类排行条形图（横向条）：按金额降序排列。
class RankBarChart extends StatelessWidget {
  final List<PieSlice> items;
  final String commodity;
  final double height;
  const RankBarChart({
    super.key,
    required this.items,
    required this.commodity,
    this.height = 200,
  });

  @override
  Widget build(BuildContext context) {
    if (items.isEmpty) {
      return SizedBox(
        height: height,
        child: Center(
          child: Text('暂无数据', style: const TextStyle(color: AppColors.sub)),
        ),
      );
    }
    final maxV = items.fold(Decimal.zero, (s, e) => s > e.value ? s : e.value);
    return SizedBox(
      height: height,
      child: ListView.builder(
        itemCount: items.length,
        itemBuilder: (_, i) {
          final item = items[i];
          final ratio = maxV == Decimal.zero
              ? 0.0
              : (item.value / maxV).toDouble();
          return Padding(
            padding: const EdgeInsets.only(bottom: 8),
            child: Row(
              children: [
                SizedBox(
                  width: 80,
                  child: Text(item.label,
                      style: const TextStyle(fontSize: 12, color: AppColors.ink),
                      overflow: TextOverflow.ellipsis),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Stack(
                    children: [
                      Container(
                        height: 20,
                        decoration: BoxDecoration(
                          color: const Color(0xFFF0F1F3),
                          borderRadius: BorderRadius.circular(4),
                        ),
                      ),
                      FractionallySizedBox(
                        widthFactor: ratio.clamp(0.0, 1.0),
                        child: Container(
                          height: 20,
                          decoration: BoxDecoration(
                            color: item.color,
                            borderRadius: BorderRadius.circular(4),
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(width: 8),
                SizedBox(
                  width: 72,
                  child: Text(
                    formatMoney(item.value.toString(), commodity),
                    style: const TextStyle(
                        fontSize: 12,
                        fontWeight: FontWeight.w600,
                        color: AppColors.sub,
                        fontFeatures: [FontFeature.tabularFigures()]),
                    textAlign: TextAlign.right,
                  ),
                ),
              ],
            ),
          );
        },
      ),
    );
  }
}

/// 紧凑金额格式化（用于 y 轴刻度，避免过长）。
String _compactMoney(double v, String commodity) {
  if (v == 0) return '0';
  final sym = currencySymbol(commodity);
  if (v >= 100000) return '$sym${(v / 10000).toStringAsFixed(0)}万';
  if (v >= 10000) return '$sym${(v / 10000).toStringAsFixed(1)}万';
  return '$sym${v.toStringAsFixed(0)}';
}
