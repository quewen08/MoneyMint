/// 关闭账户「余额未结清」拦截弹窗（纯展示，便于 VM 测试）。
///
/// 展示 409 `account_balance_not_zero` 的按币种差额；提供「转到其他账户」
/// 入口（无可用转入账户时隐藏）。
import 'package:flutter/material.dart';
import '../app/theme.dart';
import '../core/api/opening_models.dart';
import '../core/models/account.dart';
import '../utils/money.dart';

class BalanceBlockedDialog extends StatelessWidget {
  final LocalAccount account;
  final List<BalanceDiff> diffs;

  /// 是否存在可转入的资产账户；为 false 时只展示提示，不提供转出按钮。
  final bool canTransfer;

  const BalanceBlockedDialog({
    super.key,
    required this.account,
    required this.diffs,
    this.canTransfer = true,
  });

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('账户余额未结清'),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('「${account.display}」以下币种余额不为 0，无法关闭：'),
          const SizedBox(height: 10),
          for (final d in diffs)
            Padding(
              padding: const EdgeInsets.only(bottom: 4),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text(d.commodity, style: AppTheme.muted),
                  Text(
                    formatMoney(d.amount, d.commodity, signed: true),
                    style: const TextStyle(
                      fontWeight: FontWeight.w700,
                      fontFeatures: [FontFeature.tabularFigures()],
                    ),
                  ),
                ],
              ),
            ),
          const SizedBox(height: 10),
          Text(
            canTransfer
                ? '可先将余额全额转到其他资产账户，结清后再关闭。'
                : '暂无可转入的资产账户；请先新建一个资产账户再转出。',
            style: AppTheme.muted,
          ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context, BlockedAction.cancel),
          child: const Text('取消'),
        ),
        if (canTransfer)
          ElevatedButton(
            onPressed: () => Navigator.pop(context, BlockedAction.transfer),
            child: const Text('转到其他账户'),
          ),
      ],
    );
  }
}

/// 拦截弹窗的用户选择；与 account_detail 的关闭流程共用。
enum BlockedAction { cancel, transfer }
