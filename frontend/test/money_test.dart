// 金额格式化符号口径测试（FR-12 / TC-12）。
// 覆盖：负数、正数、零、负零归一、千分位、signed、未知 commodity、双重负号。
import 'package:decimal/decimal.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/utils/money.dart';

void main() {
  group('formatAmount', () {
    test('正数保留值并加千分位', () {
      expect(formatAmount('1234.5'), '1,234.50');
      expect(formatAmount('1234567.89'), '1,234,567.89');
      expect(formatAmount('0'), '0.00');
    });

    test('负数如实输出负号，千分位对负号处理正确', () {
      expect(formatAmount('-1234.5'), '-1,234.50');
      expect(formatAmount('-1234567.89'), '-1,234,567.89');
      expect(formatAmount('-25'), '-25.00');
    });

    test('负零归一为 0.00（不出现 -0.00）', () {
      expect(formatAmount('-0.00'), '0.00');
      expect(formatAmount('-0'), '0.00');
      expect(formatAmount('-0.001'), '0.00');
      expect(formatAmount('0.00'), '0.00');
    });

    test('非法输入按 0 处理', () {
      expect(formatAmount('abc'), '0.00');
      expect(formatAmount(''), '0.00');
    });
  });

  group('formatMoney', () {
    test('已知币种：负数符号前置且只出现一次', () {
      expect(formatMoney('-1234.5', 'CNY'), '-¥ 1,234.50');
      expect(formatMoney('1234.5', 'CNY'), '¥ 1,234.50');
    });

    test('signed:true 正数显 +，负数显 -，零不显 +', () {
      expect(formatMoney('1234.5', 'CNY', signed: true), '+¥ 1,234.50');
      expect(formatMoney('-1234.5', 'CNY', signed: true), '-¥ 1,234.50');
      expect(formatMoney('0', 'CNY', signed: true), '¥ 0.00');
    });

    test('不得出现双重负号（-¥ -1,234.50）', () {
      final negative = formatMoney('-1234.5', 'CNY', signed: true);
      expect(negative, isNot(contains('-¥ -')));
      expect(negative, isNot(contains('--')));
      expect(negative.split('-').length - 1, 1, reason: '负号应只出现一次');
    });

    test('未知币种：符号在金额前，币种代码在后', () {
      expect(formatMoney('-1234.5', 'XYZ'), '-1,234.50 XYZ');
      expect(formatMoney('1234.5', 'XYZ', signed: true), '+1,234.50 XYZ');
      expect(formatMoney('0', 'XYZ', signed: true), '0.00 XYZ');
    });

    test('负零按零展示，signed 时也不出 +', () {
      expect(formatMoney('-0.00', 'CNY', signed: true), '¥ 0.00');
      expect(formatMoney('-0.001', 'CNY'), '¥ 0.00');
    });
  });

  test('isBalanced 仅在每个币种合计为 0 时为真', () {
    expect(isBalanced(<String, Decimal>{}), isTrue);
    expect(isBalanced({'CNY': Decimal.zero}), isTrue);
    expect(isBalanced({'CNY': Decimal.parse('0.01')}), isFalse);
  });
}
