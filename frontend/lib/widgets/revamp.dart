/// 改版 v2 共享组件（F-1）：渐变汇总头、彩色分类图标、上浮卡、账本切换 pill。
/// PC / 移动端共用——响应式双布局下 Widget 层共享，改一处两端生效。
import 'package:flutter/material.dart';

import '../app/theme.dart';

/// 深蓝渐变汇总头：月份导航 / 标题行 + 可选账本 pill + 金额汇总行。
///
/// [bottomGap] 为头部底部预留的上浮空间（子内容用 UpliftCard 叠上来时传 34~40）。
class HeroHeader extends StatelessWidget {
  final Widget leading;
  final List<Widget> actions;
  final Widget? pill;
  final Widget summary;
  final double bottomGap;
  final double topInset;

  const HeroHeader({
    super.key,
    required this.leading,
    this.actions = const [],
    this.pill,
    required this.summary,
    this.bottomGap = 0,
    this.topInset = 8,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: const BoxDecoration(
        gradient: AppColors.heroGradient,
        borderRadius: BorderRadius.vertical(bottom: Radius.circular(30)),
      ),
      padding: EdgeInsets.fromLTRB(22, 14 + topInset, 22, 14 + bottomGap),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(child: leading),
              for (final a in actions) ...[const SizedBox(width: 8), a],
            ],
          ),
          if (pill != null) ...[const SizedBox(height: 10), pill!],
          if (bottomGap > 0) SizedBox(height: bottomGap > 0 ? 12 : 0),
          summary,
        ],
      ),
    );
  }
}

/// 头部右侧的圆角小图标按钮（半透明白底）。
class HeroAction extends StatelessWidget {
  final IconData icon;
  final String tooltip;
  final VoidCallback? onTap;
  const HeroAction({super.key, required this.icon, required this.tooltip, this.onTap});

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: Material(
        color: Colors.white.withValues(alpha: .16),
        borderRadius: BorderRadius.circular(10),
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(10),
          child: SizedBox(
            width: 32,
            height: 32,
            child: Icon(icon, size: 17, color: Colors.white),
          ),
        ),
      ),
    );
  }
}

/// 头部里的账本切换 pill：`📚 家庭账本 ▾`，点击进入账本切换。
///
/// 两种配色：
/// - 默认（[light]=false）：白色半透明底 + 白字，用于渐变头（HeroHeader）。
/// - [light]=true：浅灰底 + 深字，用于白色背景（PC 内容区 / 默认 AppBar）。
class LedgerPill extends StatelessWidget {
  final String ledgerName;
  final VoidCallback? onTap;
  final bool light;

  /// 名称过长时省略，避免撑破窄屏 AppBar 的标题行。
  final int maxChars;

  const LedgerPill({
    super.key,
    required this.ledgerName,
    this.onTap,
    this.light = false,
    this.maxChars = 12,
  });

  @override
  Widget build(BuildContext context) {
    final fg = light ? AppColors.ink : Colors.white;
    final arrowFg = light ? AppColors.sub : Colors.white70;
    final name = ledgerName.length > maxChars
        ? '${ledgerName.substring(0, maxChars)}…'
        : ledgerName;
    return Material(
      color: light ? const Color(0xFFF1F3F8) : Colors.white.withValues(alpha: .16),
      borderRadius: BorderRadius.circular(999),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(999),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 11, vertical: 4),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Text('📚', style: TextStyle(fontSize: 11)),
              const SizedBox(width: 5),
              Text(name, style: TextStyle(color: fg, fontSize: 11)),
              const SizedBox(width: 4),
              Text('▾', style: TextStyle(color: arrowFg, fontSize: 10)),
            ],
          ),
        ),
      ),
    );
  }
}

/// 彩色分类/账户图标：emoji 居中 + 淡底圆角块。
///
/// [color] 传淡底色（建议 AppColors.softOf 产出）；[big] 用于列表行（40）。
class CategoryIcon extends StatelessWidget {
  final String emoji;
  final Color color;
  final double size;
  const CategoryIcon({
    super.key,
    required this.emoji,
    required this.color,
    this.size = 38,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        color: color,
        borderRadius: BorderRadius.circular(size * .32),
      ),
      alignment: Alignment.center,
      child: Text(emoji, style: TextStyle(fontSize: size * .47)),
    );
  }
}

/// 上浮卡：负 margin 叠在渐变头上（配合 HeroHeader bottomGap 使用）。
class UpliftCard extends StatelessWidget {
  final Widget child;
  final EdgeInsetsGeometry margin;
  final EdgeInsetsGeometry padding;
  const UpliftCard({
    super.key,
    required this.child,
    this.margin = const EdgeInsets.fromLTRB(16, 0, 16, 0),
    this.padding = const EdgeInsets.all(16),
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: margin,
      padding: padding,
      decoration: BoxDecoration(
        color: AppColors.card,
        borderRadius: BorderRadius.circular(18),
        boxShadow: const [
          BoxShadow(color: Color(0x0D172044), blurRadius: 12, offset: Offset(0, 2)),
        ],
      ),
      child: child,
    );
  }
}

/// 三栏金额汇总行（支出 / 收入 / 结余），竖分隔线 + 结余染色。
class SummaryRow extends StatelessWidget {
  final String expense;
  final String income;
  final String balance;
  final bool balancePositive;
  /// 标签可配置：账单页切换历史月份时显示「11月支出」而非「本月支出」。
  final String expenseLabel;
  final String incomeLabel;
  const SummaryRow({
    super.key,
    required this.expense,
    required this.income,
    required this.balance,
    required this.balancePositive,
    this.expenseLabel = '本月支出',
    this.incomeLabel = '本月收入',
  });

  @override
  Widget build(BuildContext context) {
    final vline = Container(
        width: 1,
        height: 34,
        color: Colors.white.withValues(alpha: .22),
        margin: const EdgeInsets.symmetric(horizontal: 14));
    return Row(
      children: [
        Expanded(
          child: _col(expenseLabel, expense, 22, Colors.white),
        ),
        vline,
        Expanded(
          child: _col(incomeLabel, income, 18, Colors.white),
        ),
        vline,
        Expanded(
          child: _col('结余', balance, 16,
              balancePositive ? const Color(0xFF5FE0A8) : const Color(0xFFFF9F8A)),
        ),
      ],
    );
  }

  Widget _col(String lb, String num, double fs, Color c) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(lb, style: TextStyle(fontSize: 12, color: Colors.white.withValues(alpha: .72))),
        const SizedBox(height: 3),
        // 窄屏（320~360）下三栏金额会换行成两行、把头部撑高；
        // 用 scaleDown 等比缩小，保证永远单行完整显示。
        FittedBox(
          fit: BoxFit.scaleDown,
          alignment: Alignment.centerLeft,
          child: Text(num,
              maxLines: 1,
              style: TextStyle(
                  fontSize: fs, fontWeight: FontWeight.w700, color: c,
                  fontFeatures: const [FontFeature.tabularFigures()])),
        ),
      ],
    );
  }
}
