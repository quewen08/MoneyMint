/// 账户页（设计稿「账户」）：按 资产/负债/权益/收入/支出 分组展示，可跳转到该账户流水。
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/navigation.dart';
import '../../app/theme.dart';
import '../../core/models/account.dart';
import '../../viewmodels/ledger_controller.dart';
import '../../widgets/common.dart';
import '../transactions/transactions_screen.dart';

class AccountsScreen extends StatelessWidget {
  const AccountsScreen({super.key});

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
        if (ledger.accounts.isEmpty) {
          return const Center(
            child: Padding(
              padding: EdgeInsets.all(24),
              child: Text('还没有账户，点右下角 + 创建',
                  style: TextStyle(color: AppColors.sub)),
            ),
          );
        }
        return Center(
          child: ConstrainedBox(
            constraints: BoxConstraints(maxWidth: wide ? 900 : double.infinity),
            child: ListView(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
              children: [
                for (final type in LedgerController.typeOrder)
                  AccountGroup(
                    title: _typeLabel(type),
                    rows: ledger.accountsOfType(type).map((a) {
                      return LocalAccountRow(
                        name: a.display,
                        subtitle: a.restriction != null ? '限定 ${a.restriction}' : null,
                        balances: ledger.balances[a.uuid] ?? {},
                        onTap: () => Navigator.push(
                          context,
                          MaterialPageRoute(
                            builder: (_) => TransactionsScreen(filterAccountUuid: a.uuid),
                          ),
                        ),
                        action: _AccountMenu(account: a),
                      );
                    }).toList(),
                  ),
              ],
            ),
          ),
        );
      },
    );
  }

  String _typeLabel(String type) => switch (type) {
        'Assets' => '资产',
        'Liabilities' => '负债',
        'Equity' => '权益',
        'Income' => '收入',
        'Expenses' => '支出',
        _ => type,
      };
}

/// 账户行尾操作菜单：查看流水 / 删除账户（P1-B1）。
class _AccountMenu extends StatelessWidget {
  final LocalAccount account;
  const _AccountMenu({required this.account});

  @override
  Widget build(BuildContext context) {
    return PopupMenuButton<String>(
      tooltip: '账户操作',
      icon: const Icon(Icons.more_vert, size: 18, color: AppColors.sub),
      padding: EdgeInsets.zero,
      constraints: const BoxConstraints(minWidth: 120),
      onSelected: (v) {
        if (v == 'flow') {
          Navigator.push(
            context,
            MaterialPageRoute(
              builder: (_) => TransactionsScreen(filterAccountUuid: account.uuid),
            ),
          );
        } else if (v == 'delete') {
          _confirmDelete(context);
        }
      },
      itemBuilder: (_) => const [
        PopupMenuItem(value: 'flow', child: Text('查看流水')),
        PopupMenuItem(
          value: 'delete',
          child: Text('删除账户', style: TextStyle(color: AppColors.warn)),
        ),
      ],
    );
  }

  Future<void> _confirmDelete(BuildContext context) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('删除账户？'),
        content: Text(
            '将软删账户「${account.display}」并同步到所有端。\n'
            '引用该账户的全部交易会被一并删除，余额历史将不再显示该账户。'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false), child: const Text('取消')),
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
      await AppScope.of(context).ledger.deleteAccount(account);
      if (!context.mounted) return;
      showAppSnack(context, '账户已删除');
    } catch (e) {
      if (!context.mounted) return;
      showAppSnack(context, '删除失败: $e', warn: true);
    }
  }
}
