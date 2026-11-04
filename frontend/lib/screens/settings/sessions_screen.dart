/// 登录设备（M3）：列出当前账号的全部有效会话，支持逐个吊销 / 退出其他设备。
///
/// 安全约束：服务端只下发 SID（token 的 sha256 前缀），前端**拿不到也存不住**
/// 真实 token，所以这个页面泄不出去可用于冒充登录的凭据。
import 'package:flutter/material.dart';

import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../core/api/ledger_api.dart';
import '../../core/api/sync_models.dart';
import '../../widgets/common.dart';

class SessionsScreen extends StatefulWidget {
  const SessionsScreen({super.key});

  @override
  State<SessionsScreen> createState() => _SessionsScreenState();
}

class _SessionsScreenState extends State<SessionsScreen> {
  bool _loading = true;
  String? _error;
  List<SessionItem> _items = const [];
  String? _busyId;
  bool _bulkBusy = false;

  @override
  void initState() {
    super.initState();
    // AppScope 必须在 build/didChangeDependencies 之后取，放到首帧回调里最稳。
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
      final items = await api.listSessions();
      if (!mounted) return;
      setState(() {
        _items = items;
        _loading = false;
      });
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _error = switch (e.statusCode) {
          401 => '登录已过期，请重新登录',
          _ => '加载失败（${e.statusCode}）：${e.message}',
        };
        _loading = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _error = '无法连接服务器，登录设备需要联网才能查看';
        _loading = false;
      });
    }
  }

  Future<void> _revoke(SessionItem item) async {
    final scope = AppScope.of(context);
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(item.current ? '退出当前设备？' : '吊销这台设备？'),
        content: Text(item.current
            ? '退出后需要重新登录，账本数据不会丢失。'
            : '该设备会立即失效，需要重新输入密码登录。'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('取消')),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('确定'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;

    setState(() => _busyId = item.id);
    try {
      await scope.api.revokeSession(item.id);
      if (!mounted) return;
      if (item.current) {
        // 把自己的会话吊销了 = 退出登录，回到登录页。
        await scope.auth.logout();
        if (mounted) Navigator.pop(context);
        return;
      }
      setState(() {
        _items = _items.where((x) => x.id != item.id).toList();
        _busyId = null;
      });
      if (mounted) showAppSnack(context, '已吊销「${item.device}」');
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() => _busyId = null);
      showAppSnack(context,
          e.statusCode == 404 ? '该会话已失效' : '吊销失败：${e.message}',
          warn: true);
      if (e.statusCode == 404) await _load();
    } catch (_) {
      if (!mounted) return;
      setState(() => _busyId = null);
      showAppSnack(context, '吊销失败：网络不可用', warn: true);
    }
  }

  Future<void> _revokeOthers() async {
    final scope = AppScope.of(context);
    setState(() => _bulkBusy = true);
    try {
      final n = await scope.api.revokeOtherSessions();
      if (!mounted) return;
      setState(() {
        _items = _items.where((x) => x.current).toList();
        _bulkBusy = false;
      });
      showAppSnack(context, n > 0 ? '已退出其他 $n 台设备' : '没有其他设备');
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() => _bulkBusy = false);
      showAppSnack(context, '操作失败：${e.message}', warn: true);
    } catch (_) {
      if (!mounted) return;
      setState(() => _bulkBusy = false);
      showAppSnack(context, '操作失败：网络不可用', warn: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    final hasOthers = _items.any((x) => !x.current);
    return Scaffold(
      appBar: AppBar(
        title: const Text('登录设备'),
        actions: [
          IconButton(
            tooltip: '刷新',
            icon: const Icon(Icons.refresh),
            onPressed: _loading ? null : _load,
          ),
        ],
      ),
      body: _buildBody(hasOthers),
    );
  }

  Widget _buildBody(bool hasOthers) {
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
    final wide = MediaQuery.sizeOf(context).width >= 1024;
    return RefreshIndicator(
      onRefresh: _load,
      child: Center(
        child: ConstrainedBox(
          constraints: BoxConstraints(maxWidth: wide ? 860 : double.infinity),
          child: ListView(
            physics: const AlwaysScrollableScrollPhysics(),
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 24),
            children: [
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Icon(Icons.info_outline, size: 15, color: AppColors.sub),
                  const SizedBox(width: 6),
                  Expanded(
                    child: Text(
                      '这些设备当前可以直接打开账本。发现不认识的设备就吊销它，'
                      '对方需要重新输入密码才能登录。',
                      style: AppTheme.muted.copyWith(fontSize: 12),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              for (final s in _items) ...[
                _SessionCard(
                  item: s,
                  busy: _busyId == s.id,
                  onRevoke: () => _revoke(s),
                ),
                const SizedBox(height: 10),
              ],
              if (hasOthers) ...[
                const SizedBox(height: 4),
                OutlinedButton.icon(
                  onPressed: _bulkBusy ? null : _revokeOthers,
                  icon: _bulkBusy
                      ? const SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(strokeWidth: 2))
                      : const Icon(Icons.logout, size: 18),
                  label: const Text('退出其他设备'),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}

class _SessionCard extends StatelessWidget {
  final SessionItem item;
  final bool busy;
  final VoidCallback onRevoke;
  const _SessionCard({required this.item, required this.busy, required this.onRevoke});

  @override
  Widget build(BuildContext context) {
    return AppCard(
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.only(top: 2),
            child: Icon(
              item.current ? Icons.computer : Icons.devices_other,
              size: 20,
              color: item.current ? AppColors.blue : AppColors.sub,
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Flexible(
                      child: Text(
                        item.device,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(fontWeight: FontWeight.w600),
                      ),
                    ),
                    if (item.current) ...[
                      const SizedBox(width: 6),
                      Container(
                        padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 1),
                        decoration: BoxDecoration(
                          color: AppColors.blue.withValues(alpha: .12),
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: const Text('当前设备',
                            style: TextStyle(fontSize: 10, color: AppColors.blue)),
                      ),
                    ],
                  ],
                ),
                const SizedBox(height: 3),
                Text(
                  '登录 ${_short(item.createdAt)} · 最近活跃 ${_short(item.lastSeenAt)}',
                  style: AppTheme.muted.copyWith(fontSize: 12),
                ),
                if (item.ip.isNotEmpty)
                  Text('IP ${item.ip}',
                      style: AppTheme.muted.copyWith(fontSize: 11)),
              ],
            ),
          ),
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
                  onPressed: onRevoke,
                  child: Text(
                    item.current ? '退出' : '吊销',
                    style: TextStyle(
                      fontSize: 13,
                      color: item.current ? AppColors.warn : AppColors.blue,
                    ),
                  ),
                ),
        ],
      ),
    );
  }

  /// 只展示到分钟，够判断「是不是我刚登录的」；时区转换见 formatServerTime。
  String _short(String ts) => formatServerTime(ts);
}
