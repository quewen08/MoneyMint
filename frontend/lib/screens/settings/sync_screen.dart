/// 同步状态页（设计稿「同步状态」）：展示在线/离线状态、待同步数、立即同步。
import 'package:flutter/material.dart';
import '../../app/scope.dart';
import '../../app/theme.dart';
import '../../widgets/common.dart';

class SyncScreen extends StatelessWidget {
  /// [embedded]=true 时不包 Scaffold（作为主框架 content 渲染，AppBar 由 shell 提供）；
  /// 默认 false（作为独立全屏页被 Navigator.push 时使用，自带 AppBar）。
  final bool embedded;
  const SyncScreen({super.key, this.embedded = false});

  @override
  Widget build(BuildContext context) {
    final ledger = AppScope.of(context).ledger;
    final body = ListenableBuilder(
      listenable: ledger,
      builder: (context, _) => ListView(
        padding: const EdgeInsets.all(16),
        children: [
          AppCard(
            child: Column(
              children: [
                Icon(
                  ledger.online ? Icons.cloud_done : Icons.cloud_off,
                  size: 46,
                  color: ledger.online ? AppColors.green : AppColors.sub,
                ),
                const SizedBox(height: 8),
                Text(
                  ledger.online ? '已同步' : '离线',
                  style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 18),
                ),
                const SizedBox(height: 4),
                Text(
                  ledger.online
                      ? '上次同步：刚刚 · 待同步 ${ledger.pending} 条'
                      : '离线模式 · 数据保存在本机',
                  style: AppTheme.muted,
                ),
                const SizedBox(height: 12),
                ElevatedButton(
                  onPressed: () async {
                    final ok = await ledger.syncNow();
                    if (!context.mounted) return;
                    showAppSnack(
                      context,
                      ok
                          ? '已同步（水位 ${ledger.lastSeq}）'
                          : (ledger.lastError ??
                              '离线：本地已保存，联网后自动同步'),
                      warn: !ok,
                    );
                  },
                  child: const Text('立即同步'),
                ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          SectionTitle('同步信息'),
          AppCard(
            child: Column(
              children: [
                _InfoRow(label: '服务端水位', value: '${ledger.lastSeq}'),
                _InfoRow(label: '待推送队列', value: '${ledger.pending}'),
                _InfoRow(label: '账户数', value: '${ledger.accounts.length}'),
                _InfoRow(label: '交易数', value: '${ledger.txns.length}'),
              ],
            ),
          ),
          const SizedBox(height: 12),
          const Text('无冲突 · 采用 append-only 不可变同步协议',
              textAlign: TextAlign.center, style: AppTheme.muted),
        ],
      ),
    );
    if (embedded) return body;
    return Scaffold(
      appBar: AppBar(title: const Text('同步状态')),
      body: body,
    );
  }
}

class _InfoRow extends StatelessWidget {
  final String label;
  final String value;
  const _InfoRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 8),
        child: Row(
          children: [
            Text(label, style: AppTheme.muted),
            const Spacer(),
            Text(value, style: const TextStyle(fontWeight: FontWeight.w600)),
          ],
        ),
      );
}
