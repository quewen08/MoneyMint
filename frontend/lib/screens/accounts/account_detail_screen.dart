/// 账户详情页（0.4-C）：余额卡 + 该账户月度收支趋势 + 最近交易预览 +
/// 「查看全部流水 / 设为默认 / 关闭账户」入口。
/// 入口：账户行点击（替代原直跳流水列表）。
import 'package:decimal/decimal.dart';
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/navigation.dart';
import '../../app/theme.dart';
import '../../core/models/account.dart';
import '../../core/models/transaction.dart';
import '../../viewmodels/ledger_controller.dart';
import '../../utils/money.dart';
import '../../widgets/charts.dart';
import '../../widgets/common.dart';
import '../transactions/transactions_screen.dart';
import '../transactions/txn_detail_screen.dart';

class AccountDetailScreen extends StatelessWidget {
  final LocalAccount account;
  const AccountDetailScreen({super.key, required this.account});

  /// 该账户月度收支趋势（income=正向发生额，expense=负向发生额绝对值）。
  List<TrendPoint> _monthlyTrend(LedgerController ledger) {
    final m = <String, Map<String, Decimal>>{};
    for (final t in ledger.txns) {
      for (final p in t.postings) {
        if (p.accountUuid != account.uuid) continue;
        final amt = Decimal.tryParse(p.amount) ?? Decimal.zero;
        final month = t.date.length >= 7 ? t.date.substring(0, 7) : 'unknown';
        m.putIfAbsent(month, () => {'inc': Decimal.zero, 'exp': Decimal.zero});
        if (amt >= Decimal.zero) {
          m[month]!['inc'] = m[month]!['inc']! + amt;
        } else {
          m[month]!['exp'] = m[month]!['exp']! + (-amt);
        }
      }
    }
    final months = m.keys.toList()..sort();
    return months
        .map((mo) => TrendPoint(
              label: mo.length >= 7 ? mo.substring(5) : mo,
              income: m[mo]!['inc']!,
              expense: m[mo]!['exp']!,
            ))
        .toList();
  }

  @override
  Widget build(BuildContext context) {
    final ledger = AppScope.of(context).ledger;
    final wide = isWide(context);
    final balances = ledger.balances[account.uuid] ?? {};
    final commodity = balances.keys.isNotEmpty ? balances.keys.first : 'CNY';
    final balanceStr = balances[commodity] ?? '0.00';

    return Scaffold(
      appBar: AppBar(
        title: Text(account.display),
        actions: [
          if (account.type == 'Expenses' || account.type == 'Income')
            IconButton(
              icon: const Icon(Icons.star_border),
              tooltip: '设为默认账户',
              onPressed: () => _setDefault(context, ledger),
            ),
          if (!account.isClosed)
            IconButton(
              icon: const Icon(Icons.close, color: AppColors.warn),
              tooltip: '关闭账户',
              onPressed: () => _confirmClose(context, ledger),
            ),
        ],
      ),
      body: ListenableBuilder(
        listenable: ledger,
        builder: (context, _) {
          final points = _monthlyTrend(ledger);
          final recent = ledger.txns
              .where((t) => t.postings.any((p) => p.accountUuid == account.uuid))
              .take(8)
              .toList();
          return Center(
            child: ConstrainedBox(
              constraints: BoxConstraints(maxWidth: wide ? 900 : double.infinity),
              child: ListView(
                padding: const EdgeInsets.fromLTRB(16, 12, 16, 24),
                children: [
                  AppCard(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text('当前余额', style: AppTheme.muted),
                        const SizedBox(height: 6),
                        Text(
                          formatMoney(balanceStr, commodity),
                          style: const TextStyle(
                            fontSize: 30,
                            fontWeight: FontWeight.w800,
                            fontFeatures: [FontFeature.tabularFigures()],
                          ),
                        ),
                        const SizedBox(height: 4),
                        Text(
                          '${account.name}${account.isClosed && account.closeDate != null ? ' · 已关闭 ${account.closeDate}' : ''}',
                          style: AppTheme.muted,
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 12),
                  AppCard(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        const SectionTitle('月度收支趋势'),
                        const SizedBox(height: 8),
                        TrendLineChart(points: points, commodity: commodity),
                      ],
                    ),
                  ),
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      Expanded(
                        child: ElevatedButton.icon(
                          onPressed: () => Navigator.push(
                            context,
                            MaterialPageRoute(
                              builder: (_) =>
                                  TransactionsScreen(filterAccountUuid: account.uuid),
                            ),
                          ),
                          icon: const Icon(Icons.list_alt),
                          label: const Text('查看全部流水'),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 12),
                  AppCard(
                    padding: const EdgeInsets.symmetric(vertical: 4),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        const Padding(
                          padding: EdgeInsets.fromLTRB(14, 10, 14, 4),
                          child: SectionTitle('最近交易'),
                        ),
                        if (recent.isEmpty)
                          const Padding(
                            padding: EdgeInsets.all(16),
                            child: Text('暂无交易', style: AppTheme.muted),
                          )
                        else
                          for (var i = 0; i < recent.length; i++) ...[
                            if (i > 0) const Divider(height: 1, color: AppColors.line),
                            _DetailTxnTile(ledger: ledger, txn: recent[i]),
                          ],
                      ],
                    ),
                  ),
                ],
              ),
            ),
          );
        },
      ),
    );
  }

  Future<void> _setDefault(BuildContext context, LedgerController ledger) async {
    final kind = account.type == 'Expenses' ? 'expense' : 'income';
    try {
      await ledger.setDefaultAccount(kind, account.uuid);
      if (!context.mounted) return;
      showAppSnack(context,
          '已设为默认${account.type == 'Expenses' ? '支出' : '收入'}账户');
    } catch (e) {
      if (!context.mounted) return;
      showAppSnack(context, '设置失败: $e', warn: true);
    }
  }

  Future<void> _confirmClose(BuildContext context, LedgerController ledger) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('关闭账户？'),
        content: Text(
            '将关闭账户「${account.display}」并同步到所有端。\n'
            '历史交易全部保留，余额历史照常计入。'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false), child: const Text('取消')),
          ElevatedButton(
            style: ElevatedButton.styleFrom(backgroundColor: AppColors.warn),
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('关闭'),
          ),
        ],
      ),
    );
    if (ok != true) return;
    try {
      await ledger.closeAccount(account);
      if (!context.mounted) return;
      showAppSnack(context, '账户已关闭');
      Navigator.pop(context);
    } catch (e) {
      if (!context.mounted) return;
      showAppSnack(context, '关闭失败: $e', warn: true);
    }
  }
}

/// 详情页内联交易行（轻量复用交易页展示，避免跨文件引用私有组件）。
class _DetailTxnTile extends StatelessWidget {
  final LedgerController ledger;
  final LocalTxn txn;
  const _DetailTxnTile({required this.ledger, required this.txn});

  @override
  Widget build(BuildContext context) {
    final head = ledger.txnHeadline(txn);
    final headCommodity = txn.postings.isNotEmpty ? txn.postings.first.commodity : 'CNY';
    return ListTile(
      contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 4),
      title: Text(
        txn.description.isEmpty ? '(无说明)' : txn.description,
        style: const TextStyle(fontWeight: FontWeight.w500),
      ),
      subtitle: Text('${txn.date} · ${ledger.txnFlow(txn)}', style: AppTheme.muted),
      trailing: head != null ? AmountText(head, headCommodity) : null,
      onTap: () => Navigator.push(
        context,
        MaterialPageRoute(builder: (_) => TxnDetailScreen(txn: txn)),
      ),
    );
  }
}
