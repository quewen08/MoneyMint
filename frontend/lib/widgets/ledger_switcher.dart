/// 账本切换面板（P1-C 多账本）：各页头部账本 pill 点击后弹出。
///
/// 切换链路与 `LedgerSettingsScreen` 保持一致，不新造机制：
/// `AuthController.selectLedger` 更新当前账本与 `LedgerApi` 上下文 →
/// `LedgerController.switchToLedger` 清本地缓存并重新拉取。
/// 单账本用户点开只看到自己那一个，不会出现空面板。
import 'package:flutter/material.dart';

import '../app/scope.dart';
import '../app/theme.dart';
import '../core/api/sync_models.dart';
import '../screens/settings/ledgers_screen.dart';
import 'common.dart';

String _roleLabel(String role) => switch (role) {
      'owner' => '所有者',
      'editor' => '可编辑',
      'viewer' => '只读',
      _ => role,
    };

/// 弹出账本切换面板；选中并切换完成后提示。
///
/// 保持在页面上下文里调用（pill 的 onTap），切换期间面板自身显示 loading，
/// 完成后关闭面板再刷新界面——避免在已卸载的 context 上弹 Snack。
Future<void> showLedgerSwitcher(BuildContext context) async {
  final scope = AppScope.of(context);
  final target = await showModalBottomSheet<Ledger>(
    context: context,
    backgroundColor: Colors.transparent,
    builder: (_) => _LedgerSheet(scope: scope),
  );
  if (target == null) return;
  if (!context.mounted) return;
  showAppSnack(context, '已切换到「${target.name}」');
}

class _LedgerSheet extends StatefulWidget {
  final AppScope scope;
  const _LedgerSheet({required this.scope});

  @override
  State<_LedgerSheet> createState() => _LedgerSheetState();
}

class _LedgerSheetState extends State<_LedgerSheet> {
  int? _switching;

  Future<void> _pick(Ledger l) async {
    final auth = widget.scope.auth;
    final ledger = widget.scope.ledger;
    if (l.id == auth.currentLedgerId) {
      Navigator.pop(context);
      return;
    }
    setState(() => _switching = l.id);
    // 先切 auth 上下文（LedgerApi 的 X-Ledger-Id 随之更新），再让本地库重载。
    auth.selectLedger(l);
    await ledger.switchToLedger(l.id);
    if (!mounted) return;
    Navigator.pop(context, l);
  }

  @override
  Widget build(BuildContext context) {
    final auth = widget.scope.auth;
    return ListenableBuilder(
      listenable: auth,
      builder: (context, _) {
        final ledgers = auth.ledgers;
        return SafeArea(
          top: false,
          child: Container(
            margin: const EdgeInsets.all(10),
            padding: const EdgeInsets.fromLTRB(6, 10, 6, 6),
            decoration: BoxDecoration(
              color: AppColors.card,
              borderRadius: BorderRadius.circular(18),
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Padding(
                  padding: EdgeInsets.fromLTRB(12, 2, 12, 8),
                  child: Text('切换账本',
                      style: TextStyle(fontSize: 14, fontWeight: FontWeight.w700)),
                ),
                if (ledgers.isEmpty)
                  const Padding(
                    padding: EdgeInsets.fromLTRB(12, 6, 12, 14),
                    child: Text('还没有可用的账本', style: AppTheme.muted),
                  )
                else
                  // 账本数量通常个位数，直接铺开；账本多了再换 ListView 限高。
                  for (final l in ledgers)
                    ListTile(
                      dense: true,
                      shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(12)),
                      leading: const Text('📚', style: TextStyle(fontSize: 16)),
                      title: Text(l.name,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                              fontSize: 14, fontWeight: FontWeight.w600)),
                      subtitle: Text(_roleLabel(l.role),
                          style: const TextStyle(fontSize: 11, color: AppColors.sub)),
                      trailing: _switching == l.id
                          ? const SizedBox(
                              width: 16,
                              height: 16,
                              child: CircularProgressIndicator(strokeWidth: 2))
                          : (l.id == auth.currentLedgerId
                              ? const Icon(Icons.check_circle,
                                  size: 18, color: AppColors.blue)
                              : null),
                      onTap: _switching == null ? () => _pick(l) : null,
                    ),
                const Divider(height: 12, color: AppColors.line),
                ListTile(
                  dense: true,
                  shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12)),
                  leading: const Icon(Icons.tune, size: 18, color: AppColors.sub),
                  title: const Text('管理账本（新建 / 改名）',
                      style: TextStyle(fontSize: 13)),
                  onTap: () {
                    Navigator.pop(context);
                    Navigator.push(
                      context,
                      MaterialPageRoute(builder: (_) => const LedgersScreen()),
                    );
                  },
                ),
              ],
            ),
          ),
        );
      },
    );
  }
}
