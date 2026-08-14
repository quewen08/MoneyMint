/// 借贷平衡指示器（设计稿「记一笔 ★」核心）：实时显示剩余未平金额。
import 'package:flutter/material.dart';
import '../app/theme.dart';

class BalanceMeter extends StatelessWidget {
  final bool balanced;
  final String detail; // 不平衡时的剩余金额描述
  const BalanceMeter({super.key, required this.balanced, this.detail = ''});

  @override
  Widget build(BuildContext context) => Container(
        width: double.infinity,
        padding: const EdgeInsets.symmetric(vertical: 12, horizontal: 14),
        decoration: BoxDecoration(
          color: balanced ? const Color(0xFFE7F6EF) : const Color(0xFFFDEEE3),
          borderRadius: BorderRadius.circular(12),
        ),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              balanced ? Icons.check_circle : Icons.error_outline,
              color: balanced ? AppColors.green : AppColors.warn,
              size: 18,
            ),
            const SizedBox(width: 8),
            Text(
              balanced ? '已平衡' : '未平衡：$detail',
              style: TextStyle(
                fontWeight: FontWeight.w700,
                fontSize: 15,
                color: balanced ? AppColors.green : AppColors.warn,
              ),
            ),
          ],
        ),
      );
}
