/// 记一笔自定义数字键盘（4×4）：数字 / 退格 / 清空 / 再记 / 保存。
import 'package:flutter/material.dart';
import '../app/theme.dart';

class NumKeyboard extends StatelessWidget {
  final void Function(String token) onToken; // 0-9 / 00 / .
  final VoidCallback onBackspace;
  final VoidCallback onClear;
  final VoidCallback onAgain;
  final VoidCallback onSave;
  final bool saveEnabled;
  const NumKeyboard({
    super.key,
    required this.onToken,
    required this.onBackspace,
    required this.onClear,
    required this.onAgain,
    required this.onSave,
    this.saveEnabled = true,
  });

  Widget _key(BuildContext c, String label, VoidCallback? onTap,
      {bool primary = false, bool accent = false}) {
    return Expanded(
      child: Material(
        color: primary
            ? AppColors.blue
            : accent
                ? Colors.transparent
                : const Color(0xFFF4F5F8),
        borderRadius: BorderRadius.circular(12),
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(12),
          child: SizedBox(
            height: 46,
            child: Center(
              child: accent
                  ? Text(label,
                      style: const TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                          color: AppColors.blue))
                  : Text(label,
                      style: TextStyle(
                          fontSize: primary ? 15 : 19,
                          fontWeight: FontWeight.w500,
                          color: primary ? Colors.white : AppColors.ink)),
            ),
          ),
        ),
      ),
    );
  }

  Widget _row(List<Widget> keys) => Row(
        children: [
          for (var i = 0; i < keys.length; i++) ...[
            if (i > 0) const SizedBox(width: 7),
            Expanded(child: keys[i]),
          ]
        ],
      );

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        _row([
          _key(context, '7', () => onToken('7')),
          _key(context, '8', () => onToken('8')),
          _key(context, '9', () => onToken('9')),
          _key(context, '⌫', onBackspace),
        ]),
        const SizedBox(height: 7),
        _row([
          _key(context, '4', () => onToken('4')),
          _key(context, '5', () => onToken('5')),
          _key(context, '6', () => onToken('6')),
          _key(context, 'C', onClear),
        ]),
        const SizedBox(height: 7),
        _row([
          _key(context, '1', () => onToken('1')),
          _key(context, '2', () => onToken('2')),
          _key(context, '3', () => onToken('3')),
          _key(context, '.', () => onToken('.')),
        ]),
        const SizedBox(height: 7),
        _row([
          _key(context, '再记', onAgain, accent: true),
          _key(context, '0', () => onToken('0')),
          _key(context, '00', () => onToken('00')),
          _key(context, '保存', saveEnabled ? onSave : null, primary: true),
        ]),
      ],
    );
  }
}
