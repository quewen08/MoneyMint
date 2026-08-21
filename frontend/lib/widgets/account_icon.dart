/// 账户/分类图标组件：在彩色圆角底色上显示 emoji 图标。
/// 未设置 icon 时按账户类型回退默认 emoji；未设置 color 时按类型回退默认色。
import 'package:flutter/material.dart';
import '../core/models/account.dart';

/// 各账户类型的默认 emoji（未设置 icon 时回退）。
const Map<String, String> typeEmoji = {
  'Assets': '💰',
  'Liabilities': '💳',
  'Equity': '⚖️',
  'Income': '📥',
  'Expenses': '🧾',
};

/// 各账户类型的默认颜色（未设置 color 时回退）。
const Map<String, Color> typeColor = {
  'Assets': Color(0xFF3B6EF6),
  'Liabilities': Color(0xFFE8590C),
  'Equity': Color(0xFF868E96),
  'Income': Color(0xFF2BA471),
  'Expenses': Color(0xFF9C36B5),
};

/// 解析 hex 颜色（支持 #RGB / #RRGGBB / #AARRGGBB），非法返回 null。
Color? parseHexColor(String? hex) {
  if (hex == null) return null;
  var s = hex.trim();
  if (s.startsWith('#')) s = s.substring(1);
  if (s.length == 3) {
    s = s.split('').map((c) => c + c).join();
  }
  if (s.length == 6) s = 'FF$s';
  if (s.length != 8) return null;
  final v = int.tryParse(s, radix: 16);
  return v == null ? null : Color(v);
}

/// 账户默认色（icon 缺失时基于类型）。
Color defaultAccountColor(String type) => typeColor[type] ?? const Color(0xFF868E96);

/// 账户图标：彩色圆角方块 + emoji。
class AccountIcon extends StatelessWidget {
  final LocalAccount account;
  final double size;
  final double emojiScale;
  const AccountIcon(this.account, {super.key, this.size = 40, this.emojiScale = 0.5});

  @override
  Widget build(BuildContext context) {
    final color = parseHexColor(account.color) ?? defaultAccountColor(account.type);
    final emoji = (account.icon?.isNotEmpty == true)
        ? account.icon!
        : (typeEmoji[account.type] ?? '💼');
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        color: color.withOpacity(0.14),
        borderRadius: BorderRadius.circular(size * 0.28),
      ),
      child: Center(child: Text(emoji, style: TextStyle(fontSize: size * emojiScale))),
    );
  }
}
