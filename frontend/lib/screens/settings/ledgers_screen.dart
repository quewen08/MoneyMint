/// 账本管理页（P1-C 多账本）：列出当前用户所属账本，支持切换、创建、改名。
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../core/api/sync_models.dart';
import '../../widgets/common.dart';

class LedgersScreen extends StatefulWidget {
  const LedgersScreen({super.key});

  @override
  State<LedgersScreen> createState() => _LedgersScreenState();
}

class _LedgersScreenState extends State<LedgersScreen> {
  bool _busy = false;

  String _roleLabel(String role) => switch (role) {
        'owner' => '所有者',
        'editor' => '编辑',
        'viewer' => '只读',
        _ => role,
      };

  Future<void> _select(Ledger l) async {
    final auth = AppScope.of(context).auth;
    final ledger = AppScope.of(context).ledger;
    if (l.id == auth.currentLedgerId) return;
    setState(() => _busy = true);
    auth.selectLedger(l);
    await ledger.switchToLedger(l.id);
    if (!mounted) return;
    setState(() => _busy = false);
    showAppSnack(context, '已切换到「${l.name}」');
  }

  Future<void> _create() async {
    final name = await _promptName('新建账本', '账本名称', '');
    if (name == null || name.trim().isEmpty) return;
    final auth = AppScope.of(context).auth;
    final ledger = AppScope.of(context).ledger;
    setState(() => _busy = true);
    try {
      final nl = await auth.api.createLedger(name.trim(), 'CNY');
      await auth.refreshLedgers();
      final target = auth.ledgers.firstWhere((x) => x.id == nl.id);
      auth.selectLedger(target);
      await ledger.switchToLedger(target.id);
      if (mounted) showAppSnack(context, '已创建「${nl.name}」');
    } catch (e) {
      if (mounted) showAppSnack(context, '创建失败：$e', warn: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _rename(Ledger l) async {
    final name = await _promptName('重命名账本', '账本名称', l.name);
    if (name == null || name.trim().isEmpty) return;
    final auth = AppScope.of(context).auth;
    setState(() => _busy = true);
    try {
      await auth.api.updateLedger(l.id, name.trim(), l.defaultCommodity);
      await auth.refreshLedgers();
      if (mounted) showAppSnack(context, '已重命名为「${name.trim()}」');
    } catch (e) {
      if (mounted) showAppSnack(context, '重命名失败：$e', warn: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<String?> _promptName(String title, String label, String initial) {
    final ctrl = TextEditingController(text: initial);
    return showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(title),
        content: TextField(
          controller: ctrl,
          autofocus: true,
          decoration: InputDecoration(labelText: label),
          onSubmitted: (v) => Navigator.pop(ctx, v),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, ctrl.text),
            child: const Text('确定'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final auth = AppScope.of(context).auth;
    return Scaffold(
      appBar: AppBar(title: const Text('账本管理')),
      body: ListenableBuilder(
        listenable: auth,
        builder: (context, _) {
          final ledgers = auth.ledgers;
          final currentId = auth.currentLedgerId;
          return ListView(
            padding: const EdgeInsets.all(16),
            children: [
              if (ledgers.isEmpty)
                const EmptyState('还没有账本，创建一个吧')
              else
                AppCard(
                  child: Column(
                    children: [
                      for (var i = 0; i < ledgers.length; i++) ...[
                        if (i > 0) const Divider(height: 1, color: AppColors.line),
                        _LedgerRow(
                          name: ledgers[i].name,
                          role: _roleLabel(ledgers[i].role),
                          isCurrent: ledgers[i].id == currentId,
                          busy: _busy,
                          onTap: () => _select(ledgers[i]),
                          onRename: ledgers[i].role == 'owner'
                              ? () => _rename(ledgers[i])
                              : null,
                        ),
                      ],
                    ],
                  ),
                ),
              const SizedBox(height: 16),
              FilledButton.icon(
                onPressed: _busy ? null : _create,
                icon: const Icon(Icons.add),
                label: const Text('新建账本'),
              ),
              const SizedBox(height: 8),
              const Text(
                '提示：切换账本会重新同步该账本数据；当前账本离线未推送的变更会先尽力上传。',
                style: TextStyle(color: AppColors.sub, fontSize: 12),
              ),
            ],
          );
        },
      ),
    );
  }
}

class _LedgerRow extends StatelessWidget {
  final String name;
  final String role;
  final bool isCurrent;
  final bool busy;
  final VoidCallback onTap;
  final VoidCallback? onRename;
  const _LedgerRow({
    required this.name,
    required this.role,
    required this.isCurrent,
    required this.busy,
    required this.onTap,
    this.onRename,
  });

  @override
  Widget build(BuildContext context) => ListTile(
        contentPadding: EdgeInsets.zero,
        title: Row(
          children: [
            Flexible(child: Text(name, overflow: TextOverflow.ellipsis)),
            if (isCurrent) ...[
              const SizedBox(width: 8),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                decoration: BoxDecoration(
                  color: AppColors.blue.withOpacity(0.12),
                  borderRadius: BorderRadius.circular(10),
                ),
                child: const Text('当前',
                    style: TextStyle(fontSize: 11, color: AppColors.blue)),
              ),
            ],
          ],
        ),
        subtitle: Text(role, style: AppTheme.muted),
        trailing: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (onRename != null)
              IconButton(
                icon: const Icon(Icons.edit_outlined, size: 20),
                onPressed: busy ? null : onRename,
              ),
            if (busy)
              const SizedBox(
                width: 18,
                height: 18,
                child: CircularProgressIndicator(strokeWidth: 2),
              ),
          ],
        ),
        onTap: busy ? null : onTap,
      );
}
