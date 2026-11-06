/// 账户详情页（0.4-C）：余额卡 + 该账户月度收支趋势 + 最近交易预览 +
/// 「查看全部流水 / 设为默认 / 关闭账户」入口。
/// 入口：账户行点击（替代原直跳流水列表）。
///
/// 0.4 期初重构（T05）：关闭账户前先校验余额；余额非 0 时拦截并展示
/// 409 `account_balance_not_zero` 的按币种差额，提供「转到其他账户」入口。
/// 关闭仅在线可用，离线按钮置灰。
import 'package:decimal/decimal.dart';
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/navigation.dart';
import '../../app/theme.dart';
import '../../core/api/ledger_api.dart';
import '../../core/api/opening_models.dart';
import '../../core/models/account.dart';
import '../../core/models/posting.dart';
import '../../core/models/transaction.dart';
import '../../viewmodels/ledger_controller.dart';
import '../../utils/money.dart';
import '../../utils/opening.dart' show canSetOpening;
import '../../widgets/balance_blocked_dialog.dart';
import '../../widgets/charts.dart';
import '../../widgets/common.dart';
import '../transactions/transactions_screen.dart';
import '../transactions/txn_detail_screen.dart';
import 'account_dialog.dart';

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

    return ListenableBuilder(
      listenable: ledger,
      builder: (context, _) {
        // 关闭/同步后账户对象会更新，优先取列表里的最新记录。
        LocalAccount acc = account;
        for (final a in ledger.accounts) {
          if (a.uuid == account.uuid) {
            acc = a;
            break;
          }
        }
        final balances = ledger.balances[acc.uuid] ?? {};
        final commodity = balances.keys.isNotEmpty ? balances.keys.first : 'CNY';
        final balanceStr = balances[commodity] ?? '0.00';
        final points = _monthlyTrend(ledger);
        final recent = ledger.txns
            .where((t) => t.postings.any((p) => p.accountUuid == acc.uuid))
            .take(8)
            .toList();

        return Scaffold(
          appBar: AppBar(
            title: Text(acc.display),
            actions: [
              if (canSetOpening(acc.type) && !acc.isClosed)
                IconButton(
                  icon: const Icon(Icons.edit_note),
                  tooltip: ledger.online ? '编辑期初' : '联网后才能编辑期初',
                  onPressed: ledger.online
                      ? () => showDialog(
                            context: context,
                            builder: (_) => AccountDialog(account: acc),
                          )
                      : null,
                ),
              if (acc.type == 'Expenses' || acc.type == 'Income')
                IconButton(
                  icon: const Icon(Icons.star_border),
                  tooltip: '设为默认账户',
                  onPressed: () => _setDefault(context, ledger, acc),
                ),
              if (!acc.isClosed)
                IconButton(
                  icon: const Icon(Icons.close, color: AppColors.warn),
                  tooltip: ledger.online ? '关闭账户' : '联网后才能关闭账户',
                  onPressed: ledger.online
                      ? () async {
                          final closed =
                              await runCloseAccountFlow(context, ledger, acc);
                          if (closed && context.mounted) Navigator.pop(context);
                        }
                      : null,
                ),
            ],
          ),
          body: Center(
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
                          '${acc.name}${acc.isClosed && acc.closeDate != null ? ' · 已关闭 ${acc.closeDate}' : ''}',
                          style: AppTheme.muted,
                        ),
                        if (!ledger.online && !acc.isClosed)
                          const Padding(
                            padding: EdgeInsets.only(top: 8),
                            child: Text(
                              '离线状态下不可关闭账户，联网后重试。',
                              style: TextStyle(color: AppColors.warn, fontSize: 12),
                            ),
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
                                  TransactionsScreen(filterAccountUuid: acc.uuid),
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
          ),
        );
      },
    );
  }

  Future<void> _setDefault(
      BuildContext context, LedgerController ledger, LocalAccount acc) async {
    final kind = acc.type == 'Expenses' ? 'expense' : 'income';
    try {
      await ledger.setDefaultAccount(kind, acc.uuid);
      if (!context.mounted) return;
      showAppSnack(context, '已设为默认${acc.type == 'Expenses' ? '支出' : '收入'}账户');
    } catch (e) {
      if (!context.mounted) return;
      showAppSnack(context, '设置失败: $e', warn: true);
    }
  }
}

/// 关闭账户共用流程（详情页 / 账户列表菜单共用）。
///
/// 返回 true 表示账户已关闭。流程：
/// 1. 离线直接拒绝（按钮通常已置灰）；
/// 2. 本地余额非 0 → 展示按币种差额，可转到其他资产账户后重试；
/// 3. 服务端 409 `account_balance_not_zero` → 展示服务端差额；
/// 4. 余额结清后二次确认再关闭。
Future<bool> runCloseAccountFlow(
  BuildContext context,
  LedgerController ledger,
  LocalAccount account, {
  bool retryAfterTransfer = true,
}) async {
  if (!ledger.online) {
    showAppSnack(context, '关闭账户仅在线可用，请联网后重试', warn: true);
    return false;
  }
  LocalAccount acc = account;
  for (final a in ledger.accounts) {
    if (a.uuid == account.uuid) {
      acc = a;
      break;
    }
  }
  final diffs = _nonZeroBalances(ledger, acc.uuid);
  if (diffs.isNotEmpty) {
    final action = await showDialog<BlockedAction>(
      context: context,
      builder: (_) => BalanceBlockedDialog(
        account: acc,
        diffs: diffs,
        canTransfer: retryAfterTransfer && _transferTargets(ledger, acc).isNotEmpty,
      ),
    );
    if (action != BlockedAction.transfer || !context.mounted) return false;
    final moved = await showDialog<bool>(
      context: context,
      builder: (_) => _TransferBalanceDialog(ledger: ledger, source: acc),
    );
    if (moved != true || !context.mounted) return false;
    showAppSnack(context, '余额已转出，正在重试关闭');
    return runCloseAccountFlow(context, ledger, acc,
        retryAfterTransfer: false);
  }

  final ok = await showDialog<bool>(
    context: context,
    builder: (ctx) => AlertDialog(
      title: const Text('关闭账户？'),
      content: Text('将关闭账户「${acc.display}」并同步到所有端。\n'
          '余额已结清，历史交易全部保留。'),
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
  if (ok != true || !context.mounted) return false;

  try {
    await ledger.closeAccountOnline(acc);
    if (!context.mounted) return true;
    showAppSnack(context, '账户已关闭');
    return true;
  } on ApiException catch (e) {
    if (!context.mounted) return false;
    if (e.code == 'account_balance_not_zero' && e.balanceDiffs.isNotEmpty) {
      final action = await showDialog<BlockedAction>(
        context: context,
        builder: (_) => BalanceBlockedDialog(
          account: acc,
          diffs: e.balanceDiffs,
          canTransfer: retryAfterTransfer && _transferTargets(ledger, acc).isNotEmpty,
        ),
      );
      if (action == BlockedAction.transfer && context.mounted) {
        final moved = await showDialog<bool>(
          context: context,
          builder: (_) => _TransferBalanceDialog(
            ledger: ledger,
            source: acc,
            serverDiffs: e.balanceDiffs,
          ),
        );
        if (moved == true && context.mounted) {
          showAppSnack(context, '余额已转出，正在重试关闭');
          return runCloseAccountFlow(context, ledger, acc,
              retryAfterTransfer: false);
        }
      }
      return false;
    }
    showAppSnack(context, '关闭失败: ${e.message}', warn: true);
    return false;
  } catch (e) {
    if (!context.mounted) return false;
    showAppSnack(context, '关闭失败: $e', warn: true);
    return false;
  }
}

/// 本地按币种取非零余额（金额为账本符号）。
List<BalanceDiff> _nonZeroBalances(LedgerController ledger, String uuid) {
  final m = ledger.balances[uuid] ?? const <String, String>{};
  final out = <BalanceDiff>[];
  m.forEach((commodity, amount) {
    final d = Decimal.tryParse(amount) ?? Decimal.zero;
    if (d != Decimal.zero) {
      out.add(BalanceDiff(commodity: commodity, amount: amount));
    }
  });
  return out;
}

/// 可转入的账户：资产类、未关闭、且不是自身。
List<LocalAccount> _transferTargets(LedgerController ledger, LocalAccount source) => ledger
    .accounts
    .where((a) => a.type == 'Assets' && !a.isClosed && a.uuid != source.uuid)
    .toList();

/// 余额转出弹窗：预填全额（按币种），日期为关闭日（今日），用户选择目标资产账户。
class _TransferBalanceDialog extends StatefulWidget {
  final LedgerController ledger;
  final LocalAccount source;
  final List<BalanceDiff>? serverDiffs;

  const _TransferBalanceDialog({
    required this.ledger,
    required this.source,
    this.serverDiffs,
  });

  @override
  State<_TransferBalanceDialog> createState() => _TransferBalanceDialogState();
}

class _TransferBalanceDialogState extends State<_TransferBalanceDialog> {
  List<LocalAccount> _targets = [];
  String? _targetUuid;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    _targets = _transferTargets(widget.ledger, widget.source);
    if (_targets.isNotEmpty) _targetUuid = _targets.first.uuid;
  }

  /// 转出金额：优先使用本地余额，服务端 409 时回退到服务端差额。
  List<BalanceDiff> get _diffs {
    final local = _nonZeroBalances(widget.ledger, widget.source.uuid);
    return local.isNotEmpty ? local : (widget.serverDiffs ?? const []);
  }

  Future<void> _submit() async {
    if (_busy) return;
    final targetUuid = _targetUuid;
    if (targetUuid == null) {
      showAppSnack(context, '请选择转入账户', warn: true);
      return;
    }
    final target = _targets.firstWhere((a) => a.uuid == targetUuid);
    final diffs = _diffs;
    if (diffs.isEmpty) {
      showAppSnack(context, '余额已结清，无需转出');
      Navigator.pop(context, true);
      return;
    }
    final postings = <LocalPosting>[];
    for (final d in diffs) {
      final amount = Decimal.tryParse(d.amount) ?? Decimal.zero;
      if (amount == Decimal.zero) continue;
      // 源账户取负、目标账户取正：同币种合计为 0。
      postings.add(LocalPosting(
        accountUuid: widget.source.uuid,
        commodity: d.commodity,
        amount: (-amount).toString(),
      ));
      postings.add(LocalPosting(
        accountUuid: target.uuid,
        commodity: d.commodity,
        amount: amount.toString(),
      ));
    }
    if (postings.isEmpty) {
      showAppSnack(context, '余额为 0，无需转出');
      Navigator.pop(context, true);
      return;
    }
    setState(() => _busy = true);
    try {
      await widget.ledger.createTransaction(
        date: todayStr(),
        description: '账户余额转出: ${widget.source.display} → ${target.display}',
        postings: postings,
      );
      if (!mounted) return;
      Navigator.pop(context, true);
    } catch (e) {
      if (!mounted) return;
      showAppSnack(context, '转出失败: $e', warn: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final diffs = _diffs;
    return AlertDialog(
      title: const Text('转到其他账户'),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('将从「${widget.source.display}」转出以下余额：'),
          const SizedBox(height: 8),
          for (final d in diffs)
            Text(
              '${d.commodity} ${formatMoney(d.amount, d.commodity)}',
              style: const TextStyle(fontWeight: FontWeight.w600),
            ),
          const SizedBox(height: 12),
          DropdownButtonFormField<String>(
            value: _targetUuid,
            isExpanded: true,
            items: _targets
                .map((a) => DropdownMenuItem(value: a.uuid, child: Text(a.display)))
                .toList(),
            onChanged: _busy ? null : (v) => setState(() => _targetUuid = v),
            decoration: const InputDecoration(labelText: '转入账户（资产类）'),
          ),
          const SizedBox(height: 8),
          Text('日期：${todayStr()}（关闭日）', style: AppTheme.muted),
        ],
      ),
      actions: [
        TextButton(
          onPressed: _busy ? null : () => Navigator.pop(context, false),
          child: const Text('取消'),
        ),
        ElevatedButton(
          onPressed: _busy || _targets.isEmpty ? null : _submit,
          child: _busy
              ? const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                      strokeWidth: 2, color: Colors.white),
                )
              : const Text('确认转出'),
        ),
      ],
    );
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
