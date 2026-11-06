/// 金额格式化工具。所有金额均为定点十进制字符串，禁止浮点运算。
///
/// 符号口径（FR-12，见 docs/opening-balance-design.md §10.1）：
/// - [formatAmount] 保留金额本身的符号，只做千分位与两位小数；
/// - [formatMoney] 负号统一由本函数前置到币种符号之前，金额本体始终为绝对值，
///   避免出现 `-¥ -1,234.50` 这类双重负号；
/// - 需要展示「金额大小」的调用点必须显式调用 `abs()`，不得依赖格式化层吞符号。
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

/// 把定点十进制字符串格式化为带千分位、2 位小数的展示串（保留符号）。
///
/// - `-1234.5` → `-1,234.50`；
/// - `-0.00` 归一为 `0.00`，避免界面出现负零；
/// - 无法解析的输入按 0 处理（与既有行为一致）。
String formatAmount(String amount) {
  final d = Decimal.tryParse(amount) ?? Decimal.zero;
  final magnitude = _formatMagnitude(d.abs());
  // 负零（含四舍五入后为 0.00 的极小负数）统一归一为 0.00。
  final negative = d < Decimal.zero && magnitude != '0.00';
  return '${negative ? '-' : ''}$magnitude';
}

/// 完整展示：符号 + 金额（+ 可选币种代码）。
/// 已知币种如 `-¥ 1,234.50`；未知币种如 `-1,234.50 USD`。
///
/// [signed] 为 true 时正数显 `+`（零不显 `+`），负数始终显 `-` 且只出现一次。
String formatMoney(String amount, String commodity, {bool signed = false}) {
  final d = Decimal.tryParse(amount) ?? Decimal.zero;
  final sym = currencySymbol(commodity);
  // 金额本体取绝对值：负号只在下面统一前置，避免与 formatAmount 的负号叠加。
  final numStr = _formatMagnitude(d.abs());
  final body = sym.isNotEmpty ? '$sym $numStr' : numStr;

  // 显示为零（含负零）时不带任何符号；signed 的 `+` 也不出现在零上。
  final isDisplayZero = numStr == '0.00';
  final isNegative = d < Decimal.zero && !isDisplayZero;
  final isPositive = d > Decimal.zero && !isDisplayZero;
  final sign = isNegative ? '-' : (signed && isPositive ? '+' : '');
  final result = '$sign$body';

  if (sym.isEmpty && commodity.isNotEmpty) return '$result $commodity';
  return result;
}

/// 判断一组借贷余额是否已平（每个币种合计为 0）。
bool isBalanced(Map<String, Decimal> balances) =>
    balances.values.every((v) => v == Decimal.zero);

/// 格式化非负金额本体（千分位 + 两位小数）。调用方必须已取绝对值。
String _formatMagnitude(Decimal d) {
  final parts = d.toStringAsFixed(2).split('.');
  var intPart = parts[0];
  final dec = parts[1];
  // 兜底：四舍五入后可能出现 -0.00，归一为正零。
  if (intPart == '-0' && dec == '00') intPart = '0';
  final digits = intPart.startsWith('-') ? intPart.substring(1) : intPart;
  return '${_groupThousands(digits)}.$dec';
}

String _groupThousands(String digits) {
  final buf = StringBuffer();
  var count = 0;
  for (var i = digits.length - 1; i >= 0; i--) {
    if (count > 0 && count % 3 == 0) buf.write(',');
    buf.write(digits[i]);
    count++;
  }
  return buf.toString().split('').reversed.join('');
}
