/// 金额格式化工具。所有金额均为定点十进制字符串，禁止浮点运算。
import 'package:decimal/decimal.dart';

/// 常见币种的显示符号；未知币种返回空串（直接显示代码）。
String currencySymbol(String commodity) {
  switch (commodity) {
    case 'CNY':
      return '¥';
    case 'USD':
      return '\$';
    case 'EUR':
      return '€';
    case 'JPY':
      return '¥';
    case 'GBP':
      return '£';
    case 'HKD':
      return 'HK\$';
    default:
      return '';
  }
}

/// 把定点十进制字符串格式化为带千分位、2 位小数的展示串（不含符号）。
String formatAmount(String amount) {
  final d = Decimal.tryParse(amount) ?? Decimal.zero;
  final abs = d.abs();
  final parts = abs.toStringAsFixed(2).split('.');
  final intPart = parts[0];
  final dec = parts[1];
  final withSep = _groupThousands(intPart);
  return '$withSep.$dec';
}

String _groupThousands(String intPart) {
  final neg = intPart.startsWith('-');
  final digits = neg ? intPart.substring(1) : intPart;
  final buf = StringBuffer();
  var count = 0;
  for (var i = digits.length - 1; i >= 0; i--) {
    if (count > 0 && count % 3 == 0) buf.write(',');
    buf.write(digits[i]);
    count++;
  }
  final reversed = buf.toString().split('').reversed.join('');
  return neg ? '-$reversed' : reversed;
}

/// 完整展示：符号 + 金额（+ 可选币种代码）。
/// 已知币种如 "¥ 1,234.50"；未知币种如 "1,234.50 USD"。
/// [signed] 为 true 时负数显负号、正数显正号（用于借贷方向展示）。
String formatMoney(String amount, String commodity, {bool signed = false}) {
  final d = Decimal.tryParse(amount) ?? Decimal.zero;
  final sym = currencySymbol(commodity);
  final numStr = formatAmount(amount);
  final body = sym.isNotEmpty ? '$sym $numStr' : numStr;

  String result;
  if (signed && d != Decimal.zero) {
    final sign = d < Decimal.zero ? '-' : '+';
    result = '$sign$body';
  } else {
    result = body;
  }
  if (sym.isEmpty && commodity.isNotEmpty) return '$result $commodity';
  return result;
}

/// 判断一组借贷余额是否已平（每个币种合计为 0）。
bool isBalanced(Map<String, Decimal> balances) =>
    balances.values.every((v) => v == Decimal.zero);
