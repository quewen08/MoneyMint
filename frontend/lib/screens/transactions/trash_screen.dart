/// 交易回收站（M3）：列出已软删的交易，支持逐条恢复。
///
/// 数据只来自服务端（`GET /api/transactions/trash`）——交易被删除时本地库已移除，
/// 所以本页必须联网；离线时给出明确提示而不是假装「回收站是空的」。
/// 恢复走 `POST /api/transactions/{uuid}/restore`，服务端清 `deleted_at` 并写
/// `sync_log op=create`，本页恢复成功后立刻触发一次同步，让流水/余额/报表同步刷新。
import 'package:flutter/material.dart';

import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../core/api/ledger_api.dart';
import '../../core/api/sync_models.dart';
import '../../widgets/common.dart';

class TrashScreen extends StatefulWidget {
  const TrashScreen({super.key});

  @override
  State<TrashScreen> createState() => _TrashScreenState();
}

class _TrashScreenState extends State<TrashScreen> {
  bool _loading = true;
  String? _error;
  List<TrashItem> _items = const [];
  String? _restoring; // 正在恢复的 uuid（禁用重复点击）

  @override
  void initState() {
    super.initState();
    // 不在 initState 里直接 AppScope.of：InheritedWidget 依赖必须在 build/
    // didChangeDependencies 之后才能取（项目里已踩过一次红屏）。
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _load();
    });
  }

  Future<void> _load() async {
    final api = AppScope.of(context).api;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final items = await api.listTrash();
      if (!mounted) return;
      setState(() {
        _items = items;
        _loading = false;
      });
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _error = _mapError(e);
        _loading = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _error = '无法连接服务器，回收站需要联网才能查看';
        _loading = false;
      });
    }
  }

  /// 后端错误码 → 可读文案。
  String _mapError(ApiException e) => switch (e.statusCode) {
        401 => '登录已过期，请重新登录',
        403 => '只读成员只能查看，不能访问回收站',
        404 => '接口不存在，服务端版本可能偏旧',
        _ => '加载失败（${e.statusCode}）：${e.message}',
      };

  Future<void> _restore(TrashItem item) async {
    final scope = AppScope.of(context);
    setState(() => _restoring = item.uuid);
    try {
      await scope.api.restoreTransaction(item.uuid);
      // 恢复后服务端有了一条 create 事件，拉一次把它带回本地库。
      await scope.ledger.syncNow();
      if (!mounted) return;
      setState(() {
        _items = _items.where((x) => x.uuid != item.uuid).toList();
        _restoring = null;
      });
      showAppSnack(context, '已恢复「${item.description.isEmpty ? item.date : item.description}」');
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() => _restoring = null);
      final msg = switch (e.statusCode) {
        403 => '只读成员不能恢复交易',
        404 => '这笔交易不存在或已经被恢复',
        _ => '恢复失败：${e.message}',
      };
      showAppSnack(context, msg, warn: true);
      // 403/404 说明列表已过期，重新拉一次
      if (e.statusCode == 404) await _load();
    } catch (_) {
      if (!mounted) return;
      setState(() => _restoring = null);
      showAppSnack(context, '恢复失败：网络不可用', warn: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('回收站'),
        actions: [
          IconButton(
            tooltip: '刷新',
            icon: const Icon(Icons.refresh),
            onPressed: _loading ? null : _load,
          ),
        ],
      ),
      body: _buildBody(),
    );
  }

  Widget _buildBody() {
    if (_loading) return const Center(child: CircularProgressIndicator());
    if (_error != null) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(28),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(Icons.cloud_off, size: 42, color: AppColors.sub),
              const SizedBox(height: 10),
              Text(_error!, textAlign: TextAlign.center, style: AppTheme.muted),
              const SizedBox(height: 14),
              OutlinedButton(onPressed: _load, child: const Text('重试')),
            ],
          ),
        ),
      );
    }
    if (_items.isEmpty) {
      return const Center(
        child: Padding(
          padding: EdgeInsets.all(28),
          child: Text('回收站是空的', style: AppTheme.muted),
        ),
      );
    }

    final wide = MediaQuery.sizeOf(context).width >= 1024;
    return RefreshIndicator(
      onRefresh: _load,
      child: Center(
        child: ConstrainedBox(
          constraints: BoxConstraints(maxWidth: wide ? 900 : double.infinity),
          child: ListView.builder(
            physics: const AlwaysScrollableScrollPhysics(),
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 24),
            itemCount: _items.length + 1,
            itemBuilder: (context, i) {
              if (i == 0) return const _TrashHint();
              return _TrashRow(
                item: _items[i - 1],
                busy: _restoring == _items[i - 1].uuid,
                onRestore: () => _restore(_items[i - 1]),
              );
            },
          ),
        ),
      ),
    );
  }
}

/// 顶部说明：讲清「删除 ≠ 消失」与「恢复会重新计入」。
class _TrashHint extends StatelessWidget {
  const _TrashHint();

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Icon(Icons.info_outline, size: 15, color: AppColors.sub),
          const SizedBox(width: 6),
          Expanded(
            child: Text(
              '这里的交易不计入余额、报表与导出。恢复后会重新出现在流水与统计中；'
              '删除的交易不会自动清理，可随时回来找回。',
              style: AppTheme.muted.copyWith(fontSize: 12),
            ),
          ),
        ],
      ),
    );
  }
}

class _TrashRow extends StatelessWidget {
  final TrashItem item;
  final bool busy;
  final VoidCallback onRestore;
  const _TrashRow({required this.item, required this.busy, required this.onRestore});

  @override
  Widget build(BuildContext context) {
    final head = item.headline;
    return AppCard(
      padding: const EdgeInsets.fromLTRB(14, 10, 10, 10),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Flexible(
                      child: Text(
                        item.description.isEmpty ? '(无说明)' : item.description,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(fontWeight: FontWeight.w600),
                      ),
                    ),
                    if (item.tags.isNotEmpty) ...[
                      const SizedBox(width: 6),
                      Flexible(
                        child: Text(
                          item.tags.map((t) => '#$t').join(' '),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(fontSize: 11, color: AppColors.blue),
                        ),
                      ),
                    ],
                  ],
                ),
                const SizedBox(height: 3),
                Text(
                  '${item.date} · ${item.flow}',
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: AppTheme.muted.copyWith(fontSize: 12),
                ),
                const SizedBox(height: 2),
                Text(
                  _deletedLabel(item.deletedAt, item.createdByName),
                  style: const TextStyle(fontSize: 11, color: AppColors.warn),
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          if (head != null)
            AmountText(head.amount, head.commodity, size: 13.5),
          const SizedBox(width: 6),
          busy
              ? const Padding(
                  padding: EdgeInsets.all(10),
                  child: SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  ),
                )
              : TextButton(
                  onPressed: onRestore,
                  child: const Text('恢复', style: TextStyle(fontSize: 13)),
                ),
        ],
      ),
    );
  }

  /// 「删除于 2026-11-04 15:20（boss 记）」。
  String _deletedLabel(String deletedAt, String by) {
    final t = deletedAt.isEmpty ? '' : '删除于 ${formatServerTime(deletedAt)}';
    if (by.isEmpty) return t;
    return t.isEmpty ? '记账人 $by' : '$t · $by 记';
  }
}
