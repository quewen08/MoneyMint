/// 期初方向 / 金额 / 差额口径的纯函数测试。
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/opening_models.dart';
import 'package:frontend/utils/opening.dart';

void main() {
  group('期初方向映射', () {
    test('仅资产/负债可设置期初', () {
      expect(canSetOpening('Assets'), isTrue);
      expect(canSetOpening('Liabilities'), isTrue);
      expect(canSetOpening('Equity'), isFalse);
      expect(canSetOpening('Income'), isFalse);
      expect(canSetOpening('Expenses'), isFalse);
    });

    test('Assets 默认持有、Liabilities 默认欠款', () {
      expect(defaultOpeningDirection('Assets'), openingDirectionHold);
      expect(defaultOpeningDirection('Liabilities'), openingDirectionOwe);
    });

    test('方向决定账本符号：持有为正、欠款为负', () {
      expect(signedOpeningAmount('1000.00', openingDirectionHold), '1000');
      expect(signedOpeningAmount('1000.00', openingDirectionOwe), '-1000');
      expect(signedOpeningAmount('0', openingDirectionOwe), '0');
    });

    test('符号反推方向与绝对值（编辑态预填）', () {
      expect(openingDirectionFromSigned('-3200.00'), openingDirectionOwe);
      expect(openingDirectionFromSigned('3200.00'), openingDirectionHold);
      expect(openingDirectionFromSigned('0'), openingDirectionHold);
      expect(openingAbsoluteFromSigned('-3200.00'), '3200');
      expect(openingAbsoluteFromSigned('10.50'), '10.5');
    });
  });

  group('期初金额校验', () {
    test('接受非负定点十进制并规范化', () {
      expect(normalizeOpeningAmount('1000'), '1000');
      expect(normalizeOpeningAmount(' 10.00 '), '10');
      expect(normalizeOpeningAmount('0'), '0');
    });

    test('拒绝负数/符号/非数字/空串', () {
      expect(normalizeOpeningAmount('-1'), isNull);
      expect(normalizeOpeningAmount('+1'), isNull);
      expect(normalizeOpeningAmount('abc'), isNull);
      expect(normalizeOpeningAmount(''), isNull);
    });
  });

  group('关闭账户差额展示', () {
    test('BalanceDiff 宽松解析并格式化', () {
      final diffs = BalanceDiff.listFrom([
        {'commodity': 'CNY', 'amount': '-3200.00'},
        {'commodity': 'USD', 'amount': '12.50'},
        {'bad': true},
      ]);
      expect(diffs.length, 2);
      final text = formatBalanceDiffs(diffs);
      expect(text.contains('CNY'), isTrue);
      expect(text.contains('USD'), isTrue);
    });

    test('恒等式差额文案', () {
      const diffs = [
        IdentityDiff(commodity: 'CNY', sum: '0.01'),
        IdentityDiff(commodity: 'USD', sum: '-0.02'),
      ];
      final text = formatIdentityDiffs(diffs);
      expect(text.contains('CNY'), isTrue);
      expect(text.contains('0.01'), isTrue);
      expect(text.contains('USD'), isTrue);
    });
  });

  test('IdentityResult 解析 diffs', () {
    final r = IdentityResult.fromJson({
      'balanced': false,
      'diffs': [
        {'commodity': 'CNY', 'sum': '0.01'},
      ],
      'posting_count': 12,
    });
    expect(r.balanced, isFalse);
    expect(r.diffs.single.commodity, 'CNY');
    expect(r.postingCount, 12);
  });
}
