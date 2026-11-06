/// 账本管理页（P1-C 多账本）：列出当前用户所属账本，支持切换、创建、改名、
/// 以及 owner 修改账本期初日期（0.4 期初重构：修改会重建全部期初交易）。
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

  /// 当前账本内「活跃期初交易」数量（用于确认文案「将重建 N 笔」）。
  int _activeOpeningCount() {
    final ledger = AppScope.of(context).ledger;
    return ledger.txns
        .where((t) =>
            t.tags.contains('opening') && !ledger.reversedUuids.contains(t.uuid))
        .length;
  }

  /// 期初日期上界：不晚于今日，且不晚于最早一笔非期初交易。
  String _openingDateUpperBound() {
    final today = todayStr();
    var bound = today;
    final ledger = AppScope.of(context).ledger;
    for (final t in ledger.txns) {
      if (t.date.isEmpty || t.tags.contains('opening')) continue;
      if (bound.isEmpty || t.date.compareTo(bound) < 0) bound = t.date;
    }
    return bound;
  }

  Future<void> _editOpeningDate(Ledger l) async {
    final ledger = AppScope.of(context).ledger;
    final auth = AppScope.of(context).auth;
    if (!ledger.online) {
      showAppSnack(context, '期初日期仅在线可修改，请联网后重试', warn: true);
      return;
    }
    final bound = _openingDateUpperBound();
    final boundDate = DateTime.tryParse(bound) ?? DateTime.now();
    final first = DateTime.tryParse('2000-01-01') ?? DateTime(2000);
    final current = l.openingDate.isNotEmpty
        ? (DateTime.tryParse(l.openingDate) ?? boundDate)
        : boundDate;
    final initial = current.isAfter(boundDate) ? boundDate : current;

    final picked = await showDatePicker(
      context: context,
      initialDate: initial,
      firstDate: first,
      lastDate: boundDate,
      helpText: '不得晚于最早交易（$bound）与今日',
    );
    if (picked == null || !mounted) return;
    final newDate = picked.toIso8601String().substring(0, 10);
    if (newDate.compareTo(bound) > 0) {
      showAppSnack(context, '期初日期不得晚于最早交易（$bound）或今日', warn: true);
      return;
    }

    final count = _activeOpeningCount();
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('修改期初日期？'),
        content: Text('期初日期将从 ${l.openingDate} 调整为 $newDate。\n'
            '将重建 $count 笔期初交易（旧期初冲正后重新生成），确认继续吗？'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false), child: const Text('取消')),
          ElevatedButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('确认并重建'),
          ),
        ],
      ),
    );
    if (ok != true || !mounted) return;

    setState(() => _busy = true);
    try {
      await ledger.changeLedgerOpeningDate(l, newDate);
      await auth.refreshLedgers();
      if (mounted) {
        showAppSnack(context, '期初日期已更新为 $newDate，已重建 $count 笔期初交易');
      }
    } catch (e) {
      if (mounted) showAppSnack(context, '修改期初日期失败：$e', warn: true);
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
    final ledgerCtl = AppScope.of(context).ledger;
    return Scaffold(
      appBar: AppBar(title: const Text('账本管理')),
      body: ListenableBuilder(
        listenable: auth,
        builder: (context, _) {
          final ledgers = auth.ledgers;
          final currentId = auth.currentLedgerId;
          final current = auth.currentLedger;
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
              if (current != null && current.role == 'owner') ...[
                const SizedBox(height: 12),
                AppCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          const Expanded(
                            child: Text('期初日期',
                                style: TextStyle(fontWeight: FontWeight.w700)),
                          ),
                          Text(
                            current.openingDate.isEmpty ? '未设置' : current.openingDate,
                            style: AppTheme.muted,
                          ),
                          IconButton(
                            icon: const Icon(Icons.edit_calendar_outlined, size: 20),
                            tooltip: ledgerCtl.online
                                ? '修改期初日期'
                                : '联网后才能修改期初日期',
                            onPressed: (ledgerCtl.online && !_busy)
                                ? () => _editOpeningDate(current)
                                : null,
                          ),
                        ],
                      ),
                      const SizedBox(height: 4),
                      Text(
                        ledgerCtl.online
                            ? '约束：不得晚于最早交易（${_openingDateUpperBound()}），且不得晚于今日。'
                            : '离线状态下不可修改；联网后可调整期初日期。',
                        style: TextStyle(
                          color: ledgerCtl.online ? AppColors.sub : AppColors.warn,
                          fontSize: 12,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
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
