/// 同步状态徽标（设计稿规范：绿=已同步 / 黄=待同步 / 灰=离线）。
/// 常驻各页面右上角，点击触发同步。
import 'package:flutter/material.dart';
import '../app/theme.dart';
import '../viewmodels/ledger_controller.dart';
import 'common.dart';

class SyncBadge extends StatelessWidget {
  final SyncState state;
  final VoidCallback? onTap;
  const SyncBadge({super.key, required this.state, this.onTap});

  @override
  Widget build(BuildContext context) {
    final (color, text) = switch (state) {
      SyncState.synced => (AppColors.green, '已同步'),
      SyncState.pending => (AppColors.warn, '待同步'),
      SyncState.offline => (const Color(0xFF9AA0A6), '离线'),
    };
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(999),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
        decoration: BoxDecoration(
          color: color.withOpacity(0.12),
          borderRadius: BorderRadius.circular(999),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 8,
              height: 8,
              decoration: BoxDecoration(color: color, shape: BoxShape.circle),
            ),
            const SizedBox(width: 6),
            Text(
              text,
              style: TextStyle(fontSize: 12, color: color, fontWeight: FontWeight.w600),
            ),
          ],
        ),
      ),
    );
  }
}

class SyncBadgeFromController extends StatelessWidget {
  final LedgerController ledger;
  const SyncBadgeFromController(this.ledger, {super.key});

  @override
  Widget build(BuildContext context) => SyncBadge(
        state: ledger.syncState,
        onTap: () async {
          final ok = await ledger.syncNow();
          if (!context.mounted) return;
          showAppSnack(
            context,
            ok
                ? (ledger.pending > 0
                    ? '已同步（水位 ${ledger.lastSeq}，仍有 ${ledger.pending} 条待推送）'
                    : '已同步（水位 ${ledger.lastSeq}）')
                : '离线：本地已保存，联网后自动同步',
            warn: !ok,
          );
        },
      );
}
