/// 首页（设计稿「首页」）：净资产大字号 + 资产/负债/权益分组 + 最近交易。
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/navigation.dart';
import '../../app/theme.dart';
import '../../core/models/transaction.dart';
import '../../viewmodels/ledger_controller.dart';
import '../../utils/money.dart';
import '../../widgets/common.dart';
import '../transactions/txn_detail_screen.dart';

class DashboardScreen extends StatelessWidget {
  const DashboardScreen({super.key});

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
        final assetRows =
            ledger.accountsOfType('Assets').map(_toRow(ledger)).toList();
        final liabRows =
            ledger.accountsOfType('Liabilities').map(_toRow(ledger)).toList();
        final equityRows =
            ledger.accountsOfType('Equity').map(_toRow(ledger)).toList();
        final groups = <Widget>[
          if (wide)
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(child: AccountGroup(title: '资产', rows: assetRows)),
                const SizedBox(width: 12),
                Expanded(child: AccountGroup(title: '负债', rows: liabRows)),
                const SizedBox(width: 12),
                Expanded(child: AccountGroup(title: '权益', rows: equityRows)),
              ],
            )
          else ...[
            AccountGroup(title: '资产', rows: assetRows),
            const SizedBox(height: 8),
            AccountGroup(title: '负债', rows: liabRows),
            const SizedBox(height: 8),
            AccountGroup(title: '权益', rows: equityRows),
          ],
        ];
        return RefreshIndicator(
          onRefresh: () => ledger.syncNow().then((_) {}),
          child: SingleChildScrollView(
            child: Center(
              child: ConstrainedBox(
                constraints: BoxConstraints(maxWidth: wide ? 1100 : double.infinity),
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      _NetWorthCard(netWorth: ledger.netWorth),
                      const SizedBox(height: 16),
                      ...groups,
                      const SizedBox(height: 16),
                      const SectionTitle('最近交易'),
                      _RecentTxns(ledger: ledger),
                    ],
                  ),
                ),
              ),
            ),
          ),
        );
      },
    );
  }

  LocalAccountRow Function(dynamic) _toRow(LedgerController ledger) =>
      (a) => LocalAccountRow(
            name: a.display,
            subtitle: a.restriction != null ? '限定 ${a.restriction}' : null,
            balances: ledger.balances[a.uuid] ?? {},
          );
}

class _NetWorthCard extends StatelessWidget {
  final Map<String, String> netWorth;
  const _NetWorthCard({required this.netWorth});

  @override
  Widget build(BuildContext context) {
    final entries = netWorth.entries.toList();
    return AppCard(
      padding: const EdgeInsets.all(18),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text('净资产（资产 − 负债）', style: AppTheme.muted),
          const SizedBox(height: 6),
          if (entries.isEmpty)
            const Text('¥ 0.00',
                style: TextStyle(fontSize: 34, fontWeight: FontWeight.w800, letterSpacing: -0.5))
          else
            ...entries.map((e) => Padding(
                  padding: const EdgeInsets.only(bottom: 2),
                  child: Text(
                    formatMoney(e.value, e.key, signed: false),
                    style: const TextStyle(
                      fontSize: 34,
                      fontWeight: FontWeight.w800,
                      letterSpacing: -0.5,
                    ),
                  ),
                )),
          const SizedBox(height: 4),
          const Text('数据来自本地账本，联网后自动同步', style: AppTheme.muted),
        ],
      ),
    );
  }
}

class _RecentTxns extends StatelessWidget {
  final LedgerController ledger;
  const _RecentTxns({required this.ledger});

  @override
  Widget build(BuildContext context) {
    final recent = ledger.txns.take(5).toList();
    if (recent.isEmpty) {
      return const AppCard(child: EmptyState('还没有交易，点右下角 记一笔'));
    }
    return AppCard(
      padding: EdgeInsets.zero,
      child: Column(
        children: [
          for (var i = 0; i < recent.length; i++) ...[
            if (i > 0) const Divider(height: 1, color: AppColors.line),
            _TxnTile(ledger: ledger, txn: recent[i]),
          ],
        ],
      ),
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
    return ListTile(
      contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 4),
      title: Text(txn.description.isEmpty ? '(无说明)' : txn.description,
          style: const TextStyle(fontWeight: FontWeight.w500)),
      subtitle: Text('${txn.date} · ${ledger.txnFlow(txn)}', style: AppTheme.muted),
      trailing: head != null ? AmountText(head, headCommodity) : null,
      onTap: () => Navigator.push(
        context,
        MaterialPageRoute(builder: (_) => TxnDetailScreen(txn: txn)),
      ),
    );
  }
}
