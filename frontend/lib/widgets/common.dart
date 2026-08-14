/// 通用展示组件：卡片容器、分组标题、金额文本、账户分组、空状态、Snack 辅助。
import 'package:flutter/material.dart';
import 'package:decimal/decimal.dart';
import '../app/theme.dart';
import '../utils/money.dart';

/// 白底圆角卡片容器（统一设计稿规范）。
class AppCard extends StatelessWidget {
  final Widget child;
  final EdgeInsetsGeometry? padding;
  final VoidCallback? onTap;
  const AppCard({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.all(14),
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    // 不在 Container 上写 width: double.infinity：AppCard 常作为 ListView 的
    // child（cross axis 约束可能是 loose），width=infinity 会让内部的
    // ConstrainedBox 强制 minWidth=inf，在无界宽度下抛「BoxConstraints forces
    // an infinite width」。不写 width，让 Row/Column 的 mainAxisSize.max 自然
    // 撑满 cross axis，行为一致且更安全。
    final card = Container(
      padding: padding,
      decoration: AppTheme.cardDecoration,
      child: child,
    );
    if (onTap == null) return card;
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(14),
      child: card,
    );
  }
}

/// 分组小标题（如「资产」「负债」「最近交易」）。
class SectionTitle extends StatelessWidget {
  final String text;
  const SectionTitle(this.text, {super.key});

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.only(top: 4, bottom: 8),
        child: Text(text, style: AppTheme.sectionTitle),
      );
}

/// 金额文本：等宽数字对齐，按正负着色（负=橙/红，正=绿，零=灰）。
class AmountText extends StatelessWidget {
  final String amount; // 定点十进制字符串
  final String commodity;
  final double size;
  final bool signed;
  final FontWeight weight;
  const AmountText(
    this.amount,
    this.commodity, {
    super.key,
    this.size = 15,
    this.signed = true,
    this.weight = FontWeight.w700,
  });

  @override
  Widget build(BuildContext context) {
    final d = Decimal.tryParse(amount) ?? Decimal.zero;
    final color = d < Decimal.zero
        ? AppColors.warn
        : d > Decimal.zero
            ? AppColors.green
            : AppColors.sub;
    return Text(
      formatMoney(amount, commodity, signed: signed),
      style: TextStyle(
        fontSize: size,
        fontWeight: weight,
        color: color,
        fontFeatures: const [FontFeature.tabularFigures()],
      ),
    );
  }
}

/// 单个账户的余额行：名称 + 各币种余额（可多币种纵向排列）+ 可选操作按钮。
class AccountRow extends StatelessWidget {
  final String name;
  final String? subtitle;
  final Map<String, String> balances; // commodity -> 金额串
  final VoidCallback? onTap;
  final Widget? action; // 余额右侧操作（如删除菜单），默认无
  const AccountRow({
    super.key,
    required this.name,
    this.subtitle,
    required this.balances,
    this.onTap,
    this.action,
  });

  @override
  Widget build(BuildContext context) {
    final trailing = balances.isEmpty
        ? const Text('—', style: TextStyle(color: AppColors.sub))
        : Column(
            mainAxisAlignment: MainAxisAlignment.center,
            crossAxisAlignment: CrossAxisAlignment.end,
            children: balances.entries
                .map((e) => AmountText(e.value, e.key, size: 15))
                .toList(),
          );
    final row = Padding(
      padding: const EdgeInsets.symmetric(vertical: 10),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(name, style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w500)),
                if (subtitle != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 2),
                    child: Text(subtitle!, style: AppTheme.muted),
                  ),
              ],
            ),
          ),
          trailing,
          if (action != null) action!,
        ],
      ),
    );
    if (onTap == null) return row;
    return InkWell(onTap: onTap, child: row);
  }
}

/// 账户分组卡片：标题 + 一组账户行（行间细分隔线）。
class AccountGroup extends StatelessWidget {
  final String title;
  final List<LocalAccountRow> rows;
  const AccountGroup({super.key, required this.title, required this.rows});

  @override
  Widget build(BuildContext context) {
    if (rows.isEmpty) return const SizedBox.shrink();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SectionTitle(title),
        AppCard(
          child: Column(
            children: [
              for (var i = 0; i < rows.length; i++) ...[
                if (i > 0) const Divider(height: 1, color: AppColors.line),
                rows[i],
              ],
            ],
          ),
        ),
      ],
    );
  }
}

/// AccountGroup 的入参封装（含点击与可选操作）。
class LocalAccountRow extends StatelessWidget {
  final String name;
  final String? subtitle;
  final Map<String, String> balances;
  final VoidCallback? onTap;
  final Widget? action;
  const LocalAccountRow({
    super.key,
    required this.name,
    this.subtitle,
    required this.balances,
    this.onTap,
    this.action,
  });

  @override
  Widget build(BuildContext context) => AccountRow(
        name: name,
        subtitle: subtitle,
        balances: balances,
        onTap: onTap,
        action: action,
      );
}

/// 空状态占位。
class EmptyState extends StatelessWidget {
  final String message;
  final IconData icon;
  const EmptyState(this.message, {super.key, this.icon = Icons.inbox_outlined});

  @override
  Widget build(BuildContext context) => Center(
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 48),
          child: Column(
            children: [
              Icon(icon, size: 48, color: AppColors.sub.withOpacity(0.5)),
              const SizedBox(height: 12),
              Text(message, style: AppTheme.muted),
            ],
          ),
        ),
      );
}

/// 统一的 Snack 提示。
void showAppSnack(BuildContext context, String message, {bool warn = false}) {
  ScaffoldMessenger.of(context).showSnackBar(
    SnackBar(
      content: Text(message),
      backgroundColor: warn ? AppColors.warn : null,
    ),
  );
}

String todayStr() {
  final n = DateTime.now();
  return '${n.year}-${n.month.toString().padLeft(2, '0')}-${n.day.toString().padLeft(2, '0')}';
}
