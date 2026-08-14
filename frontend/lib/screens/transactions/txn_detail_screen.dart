/// 交易详情页（设计稿「交易详情」）：完整借贷分录（只读）+ 冲正入口。
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../core/models/transaction.dart';
import '../../viewmodels/ledger_controller.dart';
import '../../widgets/balance_meter.dart';
import '../../widgets/common.dart';

class TxnDetailScreen extends StatelessWidget {
  final LocalTxn txn;
  const TxnDetailScreen({super.key, required this.txn});

  @override
  Widget build(BuildContext context) {
    final ledger = AppScope.of(context).ledger;
    final isReversal = txn.reversedOf != null;
    final isReversed = ledger.reversedUuids.contains(txn.uuid);
    final canReverse = !isReversal && !isReversed;

    return Scaffold(
      appBar: AppBar(title: const Text('交易详情')),
      body: ListenableBuilder(
        listenable: ledger,
        builder: (context, _) => ListView(
          padding: const EdgeInsets.all(16),
          children: [
            AppCard(
              child: Column(
                children: [
                  _Row(label: '日期', value: txn.date),
                  _Row(label: '摘要', value: txn.description.isEmpty ? '(无说明)' : txn.description),
                  _Row(label: '标记', value: txn.flag, tag: true),
                ],
              ),
            ),
            const SizedBox(height: 12),
            SectionTitle('借贷分录'),
            AppCard(
              padding: EdgeInsets.zero,
              child: Column(
                children: [
                  for (var i = 0; i < txn.postings.length; i++) ...[
                    if (i > 0) const Divider(height: 1, color: AppColors.line),
                    _PostingRow(
                      name: ledger.accountName(txn.postings[i].accountUuid),
                      amount: txn.postings[i].amount,
                      commodity: txn.postings[i].commodity,
                    ),
                  ],
                ],
              ),
            ),
            const SizedBox(height: 12),
            const BalanceMeter(balanced: true),
            const SizedBox(height: 8),
            const Text('只读 · 如需更正请新增调整交易',
                textAlign: TextAlign.center, style: AppTheme.muted),
            const SizedBox(height: 16),
            if (canReverse)
              OutlinedButton.icon(
                onPressed: () => _reverse(context, ledger),
                icon: const Icon(Icons.undo),
                label: const Text('冲正该交易'),
              ),
            const SizedBox(height: 8),
            OutlinedButton.icon(
              onPressed: () => _delete(context, ledger),
              style: OutlinedButton.styleFrom(
                foregroundColor: AppColors.warn,
                side: const BorderSide(color: AppColors.warn),
              ),
              icon: const Icon(Icons.delete_outline),
              label: const Text('删除该交易'),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _delete(BuildContext context, LedgerController ledger) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('删除这笔交易？'),
        content: Text(
            '将软删「${txn.description}」并同步到所有端。\n已冲正的交易删除后，冲正记录仍会保留。'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('取消')),
          ElevatedButton(
            style: ElevatedButton.styleFrom(backgroundColor: AppColors.warn),
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('删除'),
          ),
        ],
      ),
    );
    if (ok != true) return;
    try {
      await ledger.deleteTxn(txn);
      if (!context.mounted) return;
      showAppSnack(context, '已删除交易');
      Navigator.pop(context);
    } catch (e) {
      if (!context.mounted) return;
      showAppSnack(context, '删除失败: $e', warn: true);
    }
  }

  Future<void> _reverse(BuildContext context, LedgerController ledger) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('冲正该交易？'),
        content: Text('将新增一笔反向交易抵消「${txn.description}」，原始交易保留不变。'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('取消')),
          ElevatedButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('冲正')),
        ],
      ),
    );
    if (ok != true) return;
    try {
      await ledger.reverseTxn(txn);
      if (!context.mounted) return;
      showAppSnack(context, '已生成冲正交易');
      Navigator.pop(context);
    } catch (e) {
      if (!context.mounted) return;
      showAppSnack(context, '冲正失败: $e', warn: true);
    }
  }
}

class _Row extends StatelessWidget {
  final String label;
  final String value;
  final bool tag;
  const _Row({required this.label, required this.value, this.tag = false});

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 8),
        child: Row(
          children: [
            SizedBox(width: 56, child: Text(label, style: AppTheme.muted)),
            Expanded(
              child: tag
                  ? Align(
                      alignment: Alignment.centerLeft,
                      child: Container(
                        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                        decoration: BoxDecoration(
                          color: const Color(0xFFEEF1F6),
                          borderRadius: BorderRadius.circular(999),
                        ),
                        child: Text(value, style: const TextStyle(color: AppColors.sub, fontSize: 12)),
                      ),
                    )
                  : Text(value, style: const TextStyle(fontWeight: FontWeight.w600)),
            ),
          ],
        ),
      );
}

class _PostingRow extends StatelessWidget {
  final String name;
  final String amount;
  final String commodity;
  const _PostingRow({required this.name, required this.amount, required this.commodity});

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        child: Row(
          children: [
            Expanded(child: Text(name, style: const TextStyle(fontSize: 15))),
            AmountText(amount, commodity),
          ],
        ),
      );
}
