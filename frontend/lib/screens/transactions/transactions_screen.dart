/// 流水页（设计稿「流水」）：按日期分组列出交易，可跳转详情；支持按账户过滤。
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/navigation.dart';
import '../../app/theme.dart';
import '../../core/models/transaction.dart';
import '../../viewmodels/ledger_controller.dart';
import '../../widgets/common.dart';
import 'txn_detail_screen.dart';

class TransactionsScreen extends StatelessWidget {
  final String? filterAccountUuid;
  const TransactionsScreen({super.key, this.filterAccountUuid});

  @override
  Widget build(BuildContext context) {
    final ledger = AppScope.of(context).ledger;
    final wide = isWide(context);
    return ListenableBuilder(
      listenable: ledger,
      builder: (context, _) {
        if (ledger.loading) {
          return const Center(child: CircularProgressIndicator());
        }
        final all = ledger.txns;
        final list = filterAccountUuid == null
            ? all
            : all.where((t) => t.postings.any((p) => p.accountUuid == filterAccountUuid)).toList();

        if (list.isEmpty) {
          return const Center(
            child: Padding(
              padding: EdgeInsets.all(24),
              child: Text('还没有交易，点右下角 记一笔',
                  style: TextStyle(color: AppColors.sub)),
            ),
          );
        }

        // 按日期分组（日期已倒序，这里切分连续片段）。
        final groups = <String, List<LocalTxn>>{};
        for (final t in list) {
          groups.putIfAbsent(t.date, () => []).add(t);
        }
        final dates = groups.keys.toList();

        return Center(
          child: ConstrainedBox(
            constraints: BoxConstraints(maxWidth: wide ? 900 : double.infinity),
            child: ListView(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
              children: [
            for (final date in dates) ...[
              SectionTitle(date),
              AppCard(
                padding: EdgeInsets.zero,
                child: Column(
                  children: [
                    for (var i = 0; i < groups[date]!.length; i++) ...[
                      if (i > 0) const Divider(height: 1, color: AppColors.line),
                      _TxnTile(ledger: ledger, txn: groups[date]![i]),
                    ],
                  ],
                ),
              ),
              const SizedBox(height: 12),
            ],
          ],
        ),
          ),
        );
      },
    );
  }
}

class _TxnTile extends StatelessWidget {
  final LedgerController ledger;
  final LocalTxn txn;
  const _TxnTile({required this.ledger, required this.txn});

  @override
  Widget build(BuildContext context) {
    final head = ledger.txnHeadline(txn);
    final headCommodity = txn.postings.isNotEmpty ? txn.postings.first.commodity : 'CNY';
    final isReversal = txn.reversedOf != null;
    final isReversed = ledger.reversedUuids.contains(txn.uuid);
    return ListTile(
      contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 4),
      title: Row(
        children: [
          Expanded(
            child: Text(txn.description.isEmpty ? '(无说明)' : txn.description,
                style: const TextStyle(fontWeight: FontWeight.w500)),
          ),
          if (isReversal)
            Container(
              margin: const EdgeInsets.only(left: 6),
              padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
              decoration: BoxDecoration(
                color: AppColors.warn.withOpacity(0.12),
                borderRadius: BorderRadius.circular(4),
              ),
              child: const Text('冲正',
                  style: TextStyle(fontSize: 11, color: AppColors.warn)),
            ),
          if (isReversed)
            Container(
              margin: const EdgeInsets.only(left: 6),
              padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
              decoration: BoxDecoration(
                color: Colors.grey.shade300,
                borderRadius: BorderRadius.circular(4),
              ),
              child: const Text('已冲正',
                  style: TextStyle(fontSize: 11, color: Colors.black54)),
            ),
        ],
      ),
      subtitle: Text(ledger.txnFlow(txn), style: AppTheme.muted),
      trailing: head != null ? AmountText(head, headCommodity) : null,
      onTap: () => Navigator.push(
        context,
        MaterialPageRoute(builder: (_) => TxnDetailScreen(txn: txn)),
      ),
    );
  }
}
