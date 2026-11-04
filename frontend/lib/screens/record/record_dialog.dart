/// 记一笔弹窗（PC / 宽屏）：薄壳容器，内容为 v2 共享记账面板（RecordV2Panel）。
/// 三 Tab（支出/收入/转账）+ 分类九宫格 + 数字键盘 + 分录预览 + 高级模式入口，
/// 逻辑见 record_panel.dart（沿用原 RecordDialog 的提交与离线路径，不触碰 sync 协议）。
import 'package:flutter/material.dart';
import 'record_panel.dart';

class RecordDialog extends StatelessWidget {
  const RecordDialog({super.key});

  @override
  Widget build(BuildContext context) {
    return Dialog(
      insetPadding: const EdgeInsets.all(16),
      backgroundColor: Colors.white,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(22)),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 460),
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(20, 16, 20, 16),
          child: RecordV2Panel(),
        ),
      ),
    );
  }
}
