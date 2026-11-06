/// 期初余额的纯函数口径（方向 / 符号 / 金额校验 / 差额文案）。
///
/// 服务端口径（以 `internal/service/opening.go` 为准）：
/// - 方向 `hold`（持有）→ 账本符号为正；`owe`（欠款）→ 账本符号为负；
/// - 金额始终传非负绝对值，符号完全由方向决定（与账户类型无关）。
/// - 类型只影响**默认方向**：Assets 默认持有、Liabilities 默认欠款。
library;

import 'package:decimal/decimal.dart';

import '../core/api/opening_models.dart';
import './money.dart';

/// 是否允许设置期初：仅资产 / 负债类账户（权益、收入、支出不允许）。
bool canSetOpening(String accountType) =>
    accountType == 'Assets' || accountType == 'Liabilities';

/// 账户类型的默认方向（Assets→持有，其余→欠款）。
String defaultOpeningDirection(String accountType) =>
    accountType == 'Assets' ? openingDirectionHold : openingDirectionOwe;

/// 按方向把非负绝对值转换为账本符号金额字符串。
String signedOpeningAmount(String amount, String direction) {
  final d = Decimal.tryParse(amount) ?? Decimal.zero;
  final abs = d.abs();
  if (abs == Decimal.zero) return '0';
  final text = abs.toString();
  return direction == openingDirectionOwe ? '-$text' : text;
}

/// 从账本符号金额反推方向（负→欠款，其余→持有）。
String openingDirectionFromSigned(String signedAmount) {
  final d = Decimal.tryParse(signedAmount) ?? Decimal.zero;
  return d < Decimal.zero ? openingDirectionOwe : openingDirectionHold;
}

/// 从账本符号金额反推**绝对值**（用于编辑态预填输入框）。
String openingAbsoluteFromSigned(String signedAmount) {
  final d = Decimal.tryParse(signedAmount) ?? Decimal.zero;
  return d.abs().toString();
}

/// 校验期初金额输入：必须为非负定点十进制数字。
///
/// 返回规范化后的字符串（去掉多余前导零）；非法输入返回 null。
String? normalizeOpeningAmount(String raw) {
  final text = raw.trim();
  if (text.isEmpty || text.startsWith('-') || text.startsWith('+')) return null;
  final d = Decimal.tryParse(text);
  if (d == null || d < Decimal.zero) return null;
  // 拒绝 NaN/Infinity 之类 Decimal 解析不出的形态（tryParse 已兜底）。
  return d.toString();
}

/// 关闭账户 409 差额的展示文案，如 `CNY -3,200.00、USD +12.50`。
String formatBalanceDiffs(List<BalanceDiff> diffs) {
  if (diffs.isEmpty) return '';
  return diffs
      .map((d) => '${d.commodity} ${formatAmount(d.amount)}')
      .join('、');
}

/// 恒等式差额的展示文案，如 `CNY +0.01、USD -0.02`。
String formatIdentityDiffs(List<IdentityDiff> diffs) {
  if (diffs.isEmpty) return '';
  return diffs
      .map((d) => '${d.commodity} ${formatAmount(d.sum)}')
      .join('、');
}
