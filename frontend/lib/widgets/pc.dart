/// PC 端通用展示组件：分段标题（带操作区）、指标卡、即将上线占位、简单柱状图。
import 'package:flutter/material.dart';
import '../app/theme.dart';

/// 分段标题 + 右侧操作按钮组（PC 卡片标题栏）。
class PcSectionTitle extends StatelessWidget {
  final String title;
  final List<Widget> actions;
  const PcSectionTitle(this.title, {super.key, this.actions = const []});

  @override
  Widget build(BuildContext context) => Row(
        children: [
          Expanded(
            child: Text(title,
                style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w700)),
          ),
          for (final a in actions) ...[const SizedBox(width: 8), a],
        ],
      );
}

/// PC 指标卡（标签 + 大字号数值 + 增量说明）。
class PcMetricCard extends StatelessWidget {
  final String label;
  final String value;
  final String? delta;
  final Color? deltaColor;
  final Color? valueColor;
  const PcMetricCard({
    super.key,
    required this.label,
    required this.value,
    this.delta,
    this.deltaColor,
    this.valueColor,
  });

  @override
  Widget build(BuildContext context) => Container(
        padding: const EdgeInsets.all(16),
        decoration: AppTheme.cardDecoration,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(label, style: AppTheme.muted),
            const SizedBox(height: 4),
            Text(
              value,
              style: TextStyle(
                fontSize: 22,
                fontWeight: FontWeight.w800,
                color: valueColor ?? AppColors.ink,
                fontFeatures: const [FontFeature.tabularFigures()],
              ),
            ),
            if (delta != null)
              Padding(
                padding: const EdgeInsets.only(top: 2),
                child: Text(delta!,
                    style: TextStyle(
                        fontSize: 11,
                        color: deltaColor ?? AppColors.sub)),
              ),
          ],
        ),
      );
}

/// 即将上线占位（用于尚未接后端的 PC 模块：成员管理 / 商品与价格 / 导入）。
class ComingSoon extends StatelessWidget {
  final String title;
  final String message;
  const ComingSoon(this.title, {super.key, this.message = '该模块将在后续版本开放，当前版本请先使用核心记账功能。'});

  @override
  Widget build(BuildContext context) => Container(
        padding: const EdgeInsets.all(16),
        decoration: AppTheme.cardDecoration,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            PcSectionTitle(title),
            const SizedBox(height: 16),
            Center(
              child: Padding(
                padding: const EdgeInsets.symmetric(vertical: 32),
                child: Column(
                  children: [
                    Icon(Icons.upcoming_outlined,
                        size: 48, color: AppColors.sub.withOpacity(0.5)),
                    const SizedBox(height: 12),
                    Text('即将上线', style: AppTheme.muted),
                    const SizedBox(height: 6),
                    Text(message,
                        textAlign: TextAlign.center,
                        style: const TextStyle(fontSize: 12, color: AppColors.sub)),
                  ],
                ),
              ),
            ),
          ],
        ),
      );
}

/// 极简柱状图（PC 报表占位/趋势用）：一组高度百分比的竖条。
class MiniBars extends StatelessWidget {
  final List<double> ratios; // 0~1 的高度比例
  final List<Color> colors;
  final List<String>? labels;
  final double height;
  const MiniBars({
    super.key,
    required this.ratios,
    this.colors = const [AppColors.blue],
    this.labels,
    this.height = 160,
  });

  @override
  Widget build(BuildContext context) => Container(
        height: height,
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          gradient: const LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [Color(0xFFF0F4FF), Color(0xFFE7F0FF)],
          ),
          borderRadius: BorderRadius.circular(8),
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            for (var i = 0; i < ratios.length; i++)
              Expanded(
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 2),
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.end,
                    children: [
                      Expanded(
                        child: Align(
                          alignment: Alignment.bottomCenter,
                          child: Container(
                            width: double.infinity,
                            height: (ratios[i].clamp(0, 1)) * (height - 24),
                            decoration: BoxDecoration(
                              color: i < colors.length
                                  ? colors[i]
                                  : colors.first,
                              borderRadius: BorderRadius.circular(4),
                            ),
                          ),
                        ),
                      ),
                      if (labels != null && i < labels!.length)
                        Padding(
                          padding: const EdgeInsets.only(top: 4),
                          child: Text(labels![i],
                              style: const TextStyle(
                                  fontSize: 10, color: AppColors.sub)),
                        ),
                    ],
                  ),
                ),
              ),
          ],
        ),
      );
}

/// PC 主按钮（蓝底）。
class PcButton extends StatelessWidget {
  final String label;
  final VoidCallback? onPressed;
  final bool primary;
  final IconData? icon;
  final bool danger;
  final bool ghost;
  const PcButton(
    this.label, {
    super.key,
    this.onPressed,
    this.primary = false,
    this.icon,
    this.danger = false,
    this.ghost = false,
  });

  @override
  Widget build(BuildContext context) {
    final fg = danger
        ? AppColors.warn
        : primary
            ? Colors.white
            : ghost
                ? AppColors.blue
                : AppColors.ink;
    final bg = danger || ghost
        ? Colors.white
        : primary
            ? AppColors.blue
            : const Color(0xFFEEF1F6);
    final border = (danger || ghost)
        ? BorderSide(color: danger ? AppColors.warn : AppColors.blue)
        : null;
    return ElevatedButton(
      onPressed: onPressed,
      style: ElevatedButton.styleFrom(
        backgroundColor: bg,
        foregroundColor: fg,
        elevation: 0,
        side: border,
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
        textStyle: const TextStyle(fontSize: 13, fontWeight: FontWeight.w600),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (icon != null) Icon(icon, size: 16),
          if (icon != null) const SizedBox(width: 4),
          Text(label),
        ],
      ),
    );
  }
}
