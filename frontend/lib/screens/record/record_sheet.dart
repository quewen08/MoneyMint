/// 记一笔底部弹层（移动端）：薄壳容器，内容为 v2 共享记账面板。
/// 从底部弹起（对应原型「中央 ＋ → 记一笔」），键盘弹出时跟随抬升。
import 'package:flutter/material.dart';
import 'record_panel.dart';

class RecordSheet extends StatelessWidget {
  const RecordSheet({super.key});

  @override
  Widget build(BuildContext context) {
    final insets = MediaQuery.of(context).viewInsets;
    return Padding(
      padding: EdgeInsets.only(bottom: insets.bottom),
      child: Container(
        constraints: BoxConstraints(
          maxHeight: MediaQuery.of(context).size.height * .94,
        ),
        decoration: const BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.vertical(top: Radius.circular(26)),
        ),
        padding: const EdgeInsets.fromLTRB(18, 10, 18, 14),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 40,
                height: 4,
                margin: const EdgeInsets.only(bottom: 10),
                decoration: BoxDecoration(
                    color: const Color(0xFFE3E5EC),
                    borderRadius: BorderRadius.circular(2)),
              ),
              RecordV2Panel(),
            ],
          ),
        ),
      ),
    );
  }
}
