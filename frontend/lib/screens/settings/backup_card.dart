import 'dart:html' as html;

import 'package:flutter/material.dart';

import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../core/api/ledger_api.dart';
import '../../core/api/sync_models.dart';
import '../../widgets/common.dart';

/// 数据备份卡片（M1）：仅系统管理员可见。
///
/// 设计要点：
///   - 非管理员**不渲染**（而非置灰），避免暴露"有个你看不到的功能"
///   - 备份是全库级操作，含所有账本与成员，因此不放在账本设置里
///   - 错误一律映射为可读中文，不把 HTTP 状态码或异常堆栈甩给用户
class BackupCard extends StatefulWidget {
  const BackupCard({super.key});

  @override
  State<BackupCard> createState() => _BackupCardState();
}

class _BackupCardState extends State<BackupCard> {
  List<BackupItem> _items = const [];
  bool _loading = true;
  bool _busy = false;
  bool _expanded = false;
  String? _error;
  bool _initialized = false;

  // 注意：AppScope.of(context) 只能在 build / didChangeDependencies 中调用，
  // 放 initState 会触发 "dependOnInheritedWidgetOfExactType called before initState"。
  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_initialized) return;
    _initialized = true;
    if (LedgerApi.isAdmin) {
      _load();
    } else {
      _loading = false;
    }
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final api = AppScope.of(context).api;
      final items = await api.listBackups();
      if (!mounted) return;
      setState(() {
        _items = items;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = _mapError(e);
        _loading = false;
      });
    }
  }

  Future<void> _runBackup() async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      final api = AppScope.of(context).api;
      final item = await api.runBackup();
      if (!mounted) return;
      showAppSnack(context, '备份完成：${item.name}（${item.sizeLabel}）');
      await _load();
    } catch (e) {
      if (!mounted) return;
      showAppSnack(context, _mapError(e), warn: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _download(BackupItem item) async {
    try {
      final api = AppScope.of(context).api;
      final bytes = await api.downloadBackup(item.name);
      final blob = html.Blob([bytes], 'application/octet-stream');
      final url = html.Url.createObjectUrlFromBlob(blob);
      html.AnchorElement(href: url)
        ..download = item.name
        ..click();
      html.Url.revokeObjectUrl(url);
      if (!mounted) return;
      showAppSnack(context, '已下载 ${item.name}');
    } catch (e) {
      if (!mounted) return;
      showAppSnack(context, _mapError(e), warn: true);
    }
  }

  /// 把后端错误映射成用户能懂的文案（4xx 给原因，网络类给操作建议）。
  String _mapError(Object e) {
    if (e is ApiException) {
      switch (e.statusCode) {
        case 401:
          return '登录已失效，请重新登录';
        case 403:
          return '需要管理员权限';
        case 409:
          return '已有备份正在执行，请稍后再试';
        case 503:
          return '服务端未启用备份功能';
        default:
          return e.message;
      }
    }
    return '网络异常，请检查连接后重试';
  }

  /// 相对时间：刚刚 / N 分钟前 / N 小时前 / N 天前 / 具体日期。
  String _relativeTime(DateTime t) {
    final diff = DateTime.now().difference(t);
    if (diff.inMinutes < 1) return '刚刚';
    if (diff.inMinutes < 60) return '${diff.inMinutes} 分钟前';
    if (diff.inHours < 24) return '${diff.inHours} 小时前';
    if (diff.inDays < 30) return '${diff.inDays} 天前';
    final y = t.year.toString().padLeft(4, '0');
    final m = t.month.toString().padLeft(2, '0');
    final d = t.day.toString().padLeft(2, '0');
    return '$y-$m-$d';
  }

  @override
  Widget build(BuildContext context) {
    // 非管理员：完全不渲染
    if (!LedgerApi.isAdmin) return const SizedBox.shrink();

    final succeeded = _items.where((i) => i.succeeded).toList();
    final latest = succeeded.isEmpty ? null : succeeded.first;
    final totalBytes = succeeded.fold<int>(0, (sum, i) => sum + i.size);

    return AppCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Icon(Icons.backup_outlined, size: 20, color: AppColors.blue),
              const SizedBox(width: 8),
              const Text('数据备份',
                  style: TextStyle(fontWeight: FontWeight.w700, fontSize: 16)),
              const Spacer(),
              if (_busy)
                const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
            ],
          ),
          const SizedBox(height: 6),
          if (_loading)
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 12),
              child: Text('正在读取备份状态…', style: AppTheme.muted),
            )
          else if (_error != null) ...[
            Text(_error!, style: const TextStyle(color: AppColors.warn, fontSize: 13)),
            const SizedBox(height: 8),
            Align(
              alignment: Alignment.centerLeft,
              child: TextButton(
                onPressed: _load,
                child: const Text('重试'),
              ),
            ),
          ] else ...[
            Text(
              latest == null
                  ? '尚未创建备份。建议立即备份一次，并在其他设备保存一份。'
                  : '最近备份：${_relativeTime(latest.createdAt)} · '
                      '共 ${succeeded.length} 份 · 占用 ${_sizeText(totalBytes)}',
              style: AppTheme.muted,
            ),
            const SizedBox(height: 4),
            Text(
              '备份包含全部账本的完整数据。建议把备份文件另存到其他磁盘。',
              style: AppTheme.muted.copyWith(fontSize: 12),
            ),
            const SizedBox(height: 10),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                FilledButton.tonalIcon(
                  onPressed: _busy ? null : _runBackup,
                  icon: const Icon(Icons.save_alt, size: 18),
                  label: const Text('立即备份'),
                ),
                if (latest != null)
                  OutlinedButton.icon(
                    onPressed: _busy ? null : () => _download(latest),
                    icon: const Icon(Icons.download, size: 18),
                    label: const Text('下载最新备份'),
                  ),
                if (_items.isNotEmpty)
                  TextButton(
                    onPressed: () => setState(() => _expanded = !_expanded),
                    child: Text(_expanded ? '收起列表' : '查看全部（${_items.length}）'),
                  ),
              ],
            ),
            if (_expanded) ...[
              const SizedBox(height: 8),
              const Divider(height: 1),
              for (final item in _items) _row(item),
            ],
          ],
        ],
      ),
    );
  }

  String _sizeText(int bytes) {
    if (bytes >= 1024 * 1024) return '${(bytes / 1024 / 1024).toStringAsFixed(1)} MB';
    return '${(bytes / 1024).toStringAsFixed(0)} KB';
  }

  Widget _row(BackupItem item) {
    final ok = item.succeeded;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        children: [
          Icon(
            ok ? Icons.check_circle_outline : Icons.error_outline,
            size: 16,
            color: ok ? AppColors.green : AppColors.warn,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  item.createdAt.toString().substring(0, 19),
                  style: const TextStyle(fontSize: 13),
                ),
                Text(
                  ok
                      ? '${item.isManual ? '手动' : '自动'} · ${item.sizeLabel}'
                      : '失败：${item.error ?? '未知原因'}',
                  style: AppTheme.muted.copyWith(fontSize: 12),
                ),
              ],
            ),
          ),
          if (ok)
            IconButton(
              tooltip: '下载',
              icon: const Icon(Icons.download, size: 18),
              onPressed: () => _download(item),
            ),
        ],
      ),
    );
  }
}
