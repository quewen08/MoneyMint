/// 首页恒等式告警横幅：展示按币种差额与关闭按钮。
///
/// 仅展示、不阻断业务；用户可在本次刷新内关闭，下次刷新（syncNow/init）重现。
import 'package:flutter/material.dart';
import '../app/theme.dart';
import '../core/api/opening_models.dart';
import '../utils/opening.dart';

class IdentityBanner extends StatelessWidget {
  /// 服务端/本地恒等式检查结果。
  final IdentityResult? result;

  /// 净值双算不一致等本地诊断告警文案（可选）。
  final String? warning;

  final VoidCallback onDismiss;

  const IdentityBanner({
    super.key,
    required this.result,
    required this.warning,
    required this.onDismiss,
  });

  @override
  Widget build(BuildContext context) {
    final diffs = result?.diffs ?? const <IdentityDiff>[];
    final warn = warning?.trim() ?? '';
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      padding: const EdgeInsets.fromLTRB(14, 12, 6, 12),
      decoration: BoxDecoration(
        color: const Color(0xFFFFF4E5),
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: const Color(0xFFFFD8A8)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Padding(
            padding: EdgeInsets.only(top: 2),
            child: Icon(Icons.warning_amber_rounded,
                color: AppColors.warn, size: 20),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  '账本恒等式不一致',
                  style: TextStyle(
                    color: AppColors.warn,
                    fontWeight: FontWeight.w700,
                    fontSize: 14,
                  ),
                ),
                if (diffs.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(top: 4),
                    child: Text(
                      '差额：${formatIdentityDiffs(diffs)}',
                      style: const TextStyle(
                        color: AppColors.ink,
                        fontSize: 13,
                      ),
                    ),
                  ),
                if (warn.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(top: 4),
                    child: Text(
                      warn,
                      style: const TextStyle(
                        color: AppColors.sub,
                        fontSize: 12,
                      ),
                    ),
                  ),
                const Padding(
                  padding: EdgeInsets.only(top: 4),
                  child: Text(
                    '账本仍可正常使用；请检查期初或冲正记录。',
                    style: TextStyle(color: AppColors.sub, fontSize: 12),
                  ),
                ),
              ],
            ),
          ),
          IconButton(
            key: const ValueKey('identity-banner-dismiss'),
            tooltip: '关闭提示',
            icon: const Icon(Icons.close, size: 18, color: AppColors.sub),
            onPressed: onDismiss,
          ),
        ],
      ),
    );
  }
}
